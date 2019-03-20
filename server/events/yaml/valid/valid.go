// Package valid contains the structs representing the atlantis.yaml config
// after it's been parsed and validated.
package valid

import "github.com/hashicorp/go-version"

// RepoCfg is the atlantis.yaml config after it's been parsed and validated.
type RepoCfg struct {
	// Version is the version of the atlantis YAML file. Will always be equal
	// to 2.
	Version   int
	Projects  []Project
	Workflows map[string]Workflow
	Automerge bool
}

func (c RepoCfg) GetPlanStage(workflowName string) *Stage {
	for name, flow := range c.Workflows {
		if name == workflowName {
			return flow.Plan
		}
	}
	return nil
}

func (c RepoCfg) GetApplyStage(workflowName string) *Stage {
	for name, flow := range c.Workflows {
		if name == workflowName {
			return flow.Apply
		}
	}
	return nil
}

func (c RepoCfg) FindProjectsByDirWorkspace(dir string, workspace string) []Project {
	var ps []Project
	for _, p := range c.Projects {
		if p.Dir == dir && p.Workspace == workspace {
			ps = append(ps, p)
		}
	}
	return ps
}

// FindProjectsByDir returns all projects that are in dir.
func (c RepoCfg) FindProjectsByDir(dir string) []Project {
	var ps []Project
	for _, p := range c.Projects {
		if p.Dir == dir {
			ps = append(ps, p)
		}
	}
	return ps
}

func (c RepoCfg) FindProjectByName(name string) *Project {
	for _, p := range c.Projects {
		if p.Name != nil && *p.Name == name {
			return &p
		}
	}
	return nil
}

type Project struct {
	Dir               string
	Workspace         string
	Name              *string
	Workflow          Workflow
	TerraformVersion  *version.Version
	Autoplan          Autoplan
	ApplyRequirements []string
}

// GetName returns the name of the project or an empty string if there is no
// project name.
func (p Project) GetName() string {
	if p.Name != nil {
		return *p.Name
	}
	return ""
}

type Autoplan struct {
	WhenModified []string
	Enabled      bool
}

type Stage struct {
	Steps []Step
}

type Step struct {
	StepName   string
	ExtraArgs  []string
	RunCommand []string
}

type Workflow struct {
	Apply Stage
	Plan  Stage
}

type GlobalCfg struct {
	Repos     []Repo
	Workflows map[string]Workflow
}

type Repo struct {
	ID                   string
	ApplyRequirements    []string
	Workflow             *Workflow
	AllowedOverrides     []string
	AllowCustomWorkflows *bool
}

type GlobalProjectCfg struct {
	ApplyRequirements    []string
	Workflow             Workflow
	AllowedOverrides     []string
	AllowCustomWorkflows bool
}

func (r Repo) IDMatches(otherID string) bool {
	return true
}

func (g GlobalCfg) GetProjectCfg(repoID string) GlobalProjectCfg {
	var applyReqs []string
	var workflow Workflow
	var allowedOverrides []string
	allowCustomWorkflows := false

	for _, key := range []string{"apply_requirements", "workflow", "allowed_overrides", "allow_custom_workflows"} {
		for _, repo := range g.Repos {
			if repo.IDMatches(repoID) {
				switch key {
				case "apply_requirements":
					if repo.ApplyRequirements != nil {
						applyReqs = repo.ApplyRequirements
					}
				case "workflow":
					if repo.Workflow != nil {
						workflow = *repo.Workflow
					}
				case "allowed_overrides":
					if repo.AllowedOverrides != nil {
						allowedOverrides = repo.AllowedOverrides
					}
				case "allow_custom_workflows":
					if repo.AllowCustomWorkflows != nil {
						allowCustomWorkflows = *repo.AllowCustomWorkflows
					}
				}
			}
		}
	}

	return GlobalProjectCfg{
		ApplyRequirements:    applyReqs,
		Workflow:             workflow,
		AllowedOverrides:     allowedOverrides,
		AllowCustomWorkflows: allowCustomWorkflows,
	}
}
