package events

import (
	"fmt"
	"github.com/runatlantis/atlantis/server/events/yaml/valid"
	"strings"

	"github.com/hashicorp/go-version"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/events/vcs"
	"github.com/runatlantis/atlantis/server/events/yaml"
	"github.com/runatlantis/atlantis/server/logging"
)

const (
	// DefaultRepoRelDir is the default directory we run commands in, relative
	// to the root of the repo.
	DefaultRepoRelDir = "."
	// DefaultWorkspace is the default Terraform workspace we run commands in.
	// This is also Terraform's default workspace.
	DefaultWorkspace        = "default"
	DefaultAutomergeEnabled = false
)

//go:generate pegomock generate -m --use-experimental-model-gen --package mocks -o mocks/mock_project_command_builder.go ProjectCommandBuilder

// ProjectCommandBuilder builds commands that run on individual projects.
type ProjectCommandBuilder interface {
	// BuildAutoplanCommands builds project commands that will run plan on
	// the projects determined to be modified.
	BuildAutoplanCommands(ctx *CommandContext) ([]models.ProjectCommandContext, error)
	// BuildPlanCommands builds project plan commands for this comment. If the
	// comment doesn't specify one project then there may be multiple commands
	// to be run.
	BuildPlanCommands(ctx *CommandContext, commentCommand *CommentCommand) ([]models.ProjectCommandContext, error)
	// BuildApplyCommands builds project apply commands for this comment. If the
	// comment doesn't specify one project then there may be multiple commands
	// to be run.
	BuildApplyCommands(ctx *CommandContext, commentCommand *CommentCommand) ([]models.ProjectCommandContext, error)
}

// DefaultProjectCommandBuilder implements ProjectCommandBuilder.
// This class combines the data from the comment and any repo config file or
// Atlantis server config and then generates a set of contexts.
type DefaultProjectCommandBuilder struct {
	ParserValidator   *yaml.ParserValidator
	ProjectFinder     ProjectFinder
	VCSClient         vcs.Client
	WorkingDir        WorkingDir
	WorkingDirLocker  WorkingDirLocker
	GlobalCfg         valid.GlobalCfg
	PendingPlanFinder *DefaultPendingPlanFinder
	CommentBuilder    CommentBuilder
}

// TFCommandRunner runs Terraform commands.
type TFCommandRunner interface {
	// RunCommandWithVersion runs a Terraform command using the version v.
	RunCommandWithVersion(log *logging.SimpleLogger, path string, args []string, v *version.Version, workspace string) (string, error)
}

// BuildAutoplanCommands builds project commands that will run plan on
// the projects determined to be modified.
func (p *DefaultProjectCommandBuilder) BuildAutoplanCommands(ctx *CommandContext) ([]models.ProjectCommandContext, error) {
	projCtxs, err := p.buildPlanAllCommands(ctx, nil, false)
	if err != nil {
		return nil, err
	}
	var autoplanEnabled []models.ProjectCommandContext
	for _, projCtx := range projCtxs {
		if !projCtx.AutoplanEnabled {
			ctx.Log.Debug("ignoring project at dir %q, workspace: %q because autoplan is disabled", projCtx.RepoRelDir, projCtx.Workspace)
			continue
		}
		autoplanEnabled = append(autoplanEnabled, projCtx)
	}
	return autoplanEnabled, nil
}

// BuildPlanCommands builds project plan commands for this comment. If the
// comment doesn't specify one project then there may be multiple commands
// to be run.
func (p *DefaultProjectCommandBuilder) BuildPlanCommands(ctx *CommandContext, cmd *CommentCommand) ([]models.ProjectCommandContext, error) {
	if !cmd.IsForSpecificProject() {
		return p.buildPlanAllCommands(ctx, cmd.Flags, cmd.Verbose)
	}
	pcc, err := p.buildProjectPlanCommand(ctx, cmd)
	if err != nil {
		return nil, err
	}
	return []models.ProjectCommandContext{pcc}, nil
}

// BuildApplyCommands builds project apply commands for this comment. If the
// comment doesn't specify one project then there may be multiple commands
// to be run.
func (p *DefaultProjectCommandBuilder) BuildApplyCommands(ctx *CommandContext, cmd *CommentCommand) ([]models.ProjectCommandContext, error) {
	if !cmd.IsForSpecificProject() {
		return p.buildApplyAllCommands(ctx, cmd)
	}
	pac, err := p.buildProjectApplyCommand(ctx, cmd)
	if err != nil {
		return nil, err
	}
	return []models.ProjectCommandContext{pac}, nil
}

func (p *DefaultProjectCommandBuilder) buildPlanAllCommands(ctx *CommandContext, commentFlags []string, verbose bool) ([]models.ProjectCommandContext, error) {
	// Need to lock the workspace we're about to clone to.
	workspace := DefaultWorkspace
	unlockFn, err := p.WorkingDirLocker.TryLock(ctx.BaseRepo.FullName, ctx.Pull.Num, workspace)
	if err != nil {
		ctx.Log.Warn("workspace was locked")
		return nil, err
	}
	ctx.Log.Debug("got workspace lock")
	defer unlockFn()

	// We'll need the list of modified files.
	modifiedFiles, err := p.VCSClient.GetModifiedFiles(ctx.BaseRepo, ctx.Pull)
	if err != nil {
		return nil, err
	}
	ctx.Log.Debug("%d files were modified in this pull request", len(modifiedFiles))

	repoDir, err := p.WorkingDir.Clone(ctx.Log, ctx.BaseRepo, ctx.HeadRepo, ctx.Pull, workspace)
	if err != nil {
		return nil, err
	}

	// Parse config file if it exists.
	hasRepoCfg, err := p.ParserValidator.HasConfigFile(repoDir)
	if err != nil {
		return nil, errors.Wrapf(err, "looking for %s file in %q", yaml.AtlantisYAMLFilename, repoDir)
	}

	var projCtxs []models.ProjectCommandContext
	if hasRepoCfg {
		repoCfg, err := p.ParserValidator.ParseRepoCfg(repoDir, p.GlobalCfg, ctx.BaseRepo.ID())
		if err != nil {
			return nil, err
		}
		ctx.Log.Info("successfully parsed %s file", yaml.AtlantisYAMLFilename)
		matchingProjects, err := p.ProjectFinder.DetermineProjectsViaConfig(ctx.Log, modifiedFiles, repoCfg, repoDir)
		if err != nil {
			return nil, err
		}
		ctx.Log.Info("%d projects are to be planned based on their when_modified config", len(matchingProjects))
		for _, mp := range matchingProjects {
			mergedCfg := p.GlobalCfg.MergeProjectCfg(ctx.Log, ctx.BaseRepo.ID(), mp, repoCfg)
			projCtxs = append(projCtxs, p.buildCtx(ctx, models.PlanCommand, mergedCfg, commentFlags, repoCfg.Automerge, verbose))
		}
	} else {
		ctx.Log.Info("found no %s file", yaml.AtlantisYAMLFilename)
		// If there is no config file, then we try to plan for each project that
		// was modified in the pull request.
		modifiedProjects := p.ProjectFinder.DetermineProjects(ctx.Log, modifiedFiles, ctx.BaseRepo.FullName, repoDir)
		ctx.Log.Info("automatically determined that there were %d projects modified in this pull request: %s", len(modifiedProjects), modifiedProjects)
		for _, mp := range modifiedProjects {
			pCfg := p.GlobalCfg.DefaultProjCfg(ctx.BaseRepo.ID(), mp.Path, DefaultWorkspace)
			projCtxs = append(projCtxs, p.buildCtx(ctx, models.PlanCommand, pCfg, commentFlags, DefaultAutomergeEnabled, verbose))
		}
	}

	return projCtxs, nil
}

func (p *DefaultProjectCommandBuilder) buildProjectPlanCommand(ctx *CommandContext, cmd *CommentCommand) (models.ProjectCommandContext, error) {
	workspace := DefaultWorkspace
	if cmd.Workspace != "" {
		workspace = cmd.Workspace
	}

	var pcc models.ProjectCommandContext
	ctx.Log.Debug("building plan command")
	unlockFn, err := p.WorkingDirLocker.TryLock(ctx.BaseRepo.FullName, ctx.Pull.Num, workspace)
	if err != nil {
		return pcc, err
	}
	defer unlockFn()

	ctx.Log.Debug("cloning repository")
	repoDir, err := p.WorkingDir.Clone(ctx.Log, ctx.BaseRepo, ctx.HeadRepo, ctx.Pull, workspace)
	if err != nil {
		return pcc, err
	}

	repoRelDir := DefaultRepoRelDir
	if cmd.RepoRelDir != "" {
		repoRelDir = cmd.RepoRelDir
	}

	return p.buildProjectCommandCtx(ctx, models.PlanCommand, cmd.ProjectName, cmd.Flags, repoDir, repoRelDir, workspace, cmd.Verbose)
}

func (p *DefaultProjectCommandBuilder) buildApplyAllCommands(ctx *CommandContext, commentCmd *CommentCommand) ([]models.ProjectCommandContext, error) {
	// lock all dirs in this pull request
	unlockFn, err := p.WorkingDirLocker.TryLockPull(ctx.BaseRepo.FullName, ctx.Pull.Num)
	if err != nil {
		return nil, err
	}
	defer unlockFn()

	pullDir, err := p.WorkingDir.GetPullDir(ctx.BaseRepo, ctx.Pull)
	if err != nil {
		return nil, err
	}

	plans, err := p.PendingPlanFinder.Find(pullDir)
	if err != nil {
		return nil, err
	}

	var cmds []models.ProjectCommandContext
	for _, plan := range plans {
		cmd, err := p.buildProjectCommandCtx(ctx, models.ApplyCommand, commentCmd.ProjectName, commentCmd.Flags, plan.RepoDir, plan.RepoRelDir, plan.Workspace, commentCmd.Verbose)
		if err != nil {
			return nil, errors.Wrapf(err, "building command for dir %q", plan.RepoRelDir)
		}
		cmds = append(cmds, cmd)
	}
	return cmds, nil
}

func (p *DefaultProjectCommandBuilder) buildProjectApplyCommand(ctx *CommandContext, cmd *CommentCommand) (models.ProjectCommandContext, error) {
	workspace := DefaultWorkspace
	if cmd.Workspace != "" {
		workspace = cmd.Workspace
	}

	var projCtx models.ProjectCommandContext
	unlockFn, err := p.WorkingDirLocker.TryLock(ctx.BaseRepo.FullName, ctx.Pull.Num, workspace)
	if err != nil {
		return projCtx, err
	}
	defer unlockFn()

	repoDir, err := p.WorkingDir.GetWorkingDir(ctx.BaseRepo, ctx.Pull, workspace)
	if err != nil {
		return projCtx, err
	}

	repoRelDir := DefaultRepoRelDir
	if cmd.RepoRelDir != "" {
		repoRelDir = cmd.RepoRelDir
	}

	return p.buildProjectCommandCtx(ctx, models.ApplyCommand, cmd.ProjectName, cmd.Flags, repoDir, repoRelDir, workspace, cmd.Verbose)
}

func (p *DefaultProjectCommandBuilder) buildProjectCommandCtx(
	ctx *CommandContext,
	cmd models.CommandName,
	projectName string,
	commentFlags []string,
	repoDir string,
	repoRelDir string,
	workspace string,
	verbose bool) (models.ProjectCommandContext, error) {
	var projCfg valid.MergedProjectCfg
	projCfgPtr, repoCfgPtr, err := p.getCfg(ctx, projectName, repoRelDir, workspace, repoDir)
	if err != nil {
		return models.ProjectCommandContext{}, err
	}
	if projCfgPtr != nil {
		projCfg = p.GlobalCfg.MergeProjectCfg(ctx.Log, ctx.BaseRepo.ID(), *projCfgPtr, *repoCfgPtr)
	} else {
		projCfg = p.GlobalCfg.DefaultProjCfg(ctx.BaseRepo.ID(), repoRelDir, workspace)
	}

	if err := p.validateWorkspaceAllowed(repoCfgPtr, repoRelDir, workspace); err != nil {
		return models.ProjectCommandContext{}, err
	}

	automerge := DefaultAutomergeEnabled
	if repoCfgPtr != nil {
		automerge = repoCfgPtr.Automerge
	}
	return p.buildCtx(ctx, cmd, projCfg, commentFlags, automerge, verbose), nil
}

func (p *DefaultProjectCommandBuilder) getCfg(ctx *CommandContext, projectName string, dir string, workspace string, repoDir string) (projectCfg *valid.Project, repoCfg *valid.Config, err error) {
	hasConfigFile, err := p.ParserValidator.HasConfigFile(repoDir)
	if err != nil {
		err = errors.Wrapf(err, "looking for %s file in %q", yaml.AtlantisYAMLFilename, repoDir)
		return
	}
	if !hasConfigFile {
		if projectName != "" {
			err = fmt.Errorf("cannot specify a project name unless an %s file exists to configure projects", yaml.AtlantisYAMLFilename)
			return
		}
		return
	}

	var repoConfig valid.Config
	repoConfig, err = p.ParserValidator.ParseRepoCfg(repoDir, p.GlobalCfg, ctx.BaseRepo.ID())
	if err != nil {
		return
	}
	repoCfg = &repoConfig

	// If they've specified a project by name we look it up. Otherwise we
	// use the dir and workspace.
	if projectName != "" {
		projectCfg = repoCfg.FindProjectByName(projectName)
		if projectCfg == nil {
			err = fmt.Errorf("no project with name %q is defined in %s", projectName, yaml.AtlantisYAMLFilename)
			return
		}
		return
	}

	projCfgs := repoCfg.FindProjectsByDirWorkspace(dir, workspace)
	if len(projCfgs) == 0 {
		return
	}
	if len(projCfgs) > 1 {
		err = fmt.Errorf("must specify project name: more than one project defined in %s matched dir: %q workspace: %q", yaml.AtlantisYAMLFilename, dir, workspace)
		return
	}
	projectCfg = &projCfgs[0]
	return
}

// validateWorkspaceAllowed returns an error if there are projects configured
// in globalCfg for repoRelDir and none of those projects use workspace.
func (p *DefaultProjectCommandBuilder) validateWorkspaceAllowed(globalCfg *valid.Config, repoRelDir string, workspace string) error {
	if globalCfg == nil {
		return nil
	}

	projects := globalCfg.FindProjectsByDir(repoRelDir)

	// If that directory doesn't have any projects configured then we don't
	// enforce workspace names.
	if len(projects) == 0 {
		return nil
	}

	var configuredSpaces []string
	for _, p := range projects {
		if p.Workspace == workspace {
			return nil
		}
		configuredSpaces = append(configuredSpaces, p.Workspace)
	}

	return fmt.Errorf(
		"running commands in workspace %q is not allowed because this"+
			" directory is only configured for the following workspaces: %s",
		workspace,
		strings.Join(configuredSpaces, ", "),
	)
}

func (p *DefaultProjectCommandBuilder) buildCtx(ctx *CommandContext,
	cmd models.CommandName,
	projCfg valid.MergedProjectCfg,
	commentArgs []string,
	automergeEnabled bool,
	verbose bool) models.ProjectCommandContext {

	var steps []valid.Step
	switch cmd {
	case models.PlanCommand:
		steps = projCfg.Workflow.Plan.Steps
	case models.ApplyCommand:
		steps = projCfg.Workflow.Apply.Steps
	}

	return models.ProjectCommandContext{
		ApplyCmd:          p.CommentBuilder.BuildApplyComment(projCfg.RepoRelDir, projCfg.Workspace, projCfg.Name),
		BaseRepo:          ctx.BaseRepo,
		CommentArgs:       commentArgs,
		AutomergeEnabled:  automergeEnabled,
		AutoplanEnabled:   projCfg.AutoplanEnabled,
		Steps:             steps,
		HeadRepo:          ctx.HeadRepo,
		Log:               ctx.Log,
		PullMergeable:     ctx.PullMergeable,
		Pull:              ctx.Pull,
		ProjectName:       projCfg.Name,
		ApplyRequirements: projCfg.ApplyRequirements,
		RePlanCmd:         p.CommentBuilder.BuildPlanComment(projCfg.RepoRelDir, projCfg.Workspace, projCfg.Name, commentArgs),
		RepoRelDir:        projCfg.RepoRelDir,
		TerraformVersion:  projCfg.TerraformVersion,
		User:              ctx.User,
		Verbose:           verbose,
		Workspace:         projCfg.Workspace,
	}
}
