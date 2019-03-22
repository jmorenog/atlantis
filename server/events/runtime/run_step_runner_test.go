package runtime_test

import (
	"strings"
	"testing"

	"github.com/hashicorp/go-version"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/events/runtime"
	"github.com/runatlantis/atlantis/server/logging"
	. "github.com/runatlantis/atlantis/testing"
)

func TestRunStepRunner_Run(t *testing.T) {
	cases := []struct {
		Command string
		ExpOut  string
		ExpErr  string
	}{
		{
			Command: "",
			ExpErr:  "no commands for run step",
		},
		{
			Command: "echo hi",
			ExpOut:  "hi\n",
		},
		{
			Command: "echo hi >> file && cat file",
			ExpOut:  "hi\n",
		},
		{
			Command: "lkjlkj",
			ExpErr:  "exit status 127: running \"lkjlkj\" in",
		},
		{
			Command: "echo workspace=$WORKSPACE version=$ATLANTIS_TERRAFORM_VERSION dir=$DIR planfile=$PLANFILE",
			ExpOut:  "workspace=myworkspace version=0.11.0 dir=$DIR planfile=$DIR/myworkspace.tfplan\n",
		},
		{
			Command: "echo base_repo_name=$BASE_REPO_NAME base_repo_owner=$BASE_REPO_OWNER head_repo_name=$HEAD_REPO_NAME head_repo_owner=$HEAD_REPO_OWNER head_branch_name=$HEAD_BRANCH_NAME base_branch_name=$BASE_BRANCH_NAME pull_num=$PULL_NUM pull_author=$PULL_AUTHOR",
			ExpOut:  "base_repo_name=basename base_repo_owner=baseowner head_repo_name=headname head_repo_owner=headowner head_branch_name=add-feat base_branch_name=master pull_num=2 pull_author=acme\n",
		},
		{
			Command: "echo user_name=$USER_NAME",
			ExpOut:  "user_name=acme-user\n",
		},
	}

	projVersion, err := version.NewVersion("v0.11.0")
	Ok(t, err)
	defaultVersion, _ := version.NewVersion("0.8")
	r := runtime.RunStepRunner{
		DefaultTFVersion: defaultVersion,
	}
	ctx := models.ProjectCommandContext{
		BaseRepo: models.Repo{
			Name:  "basename",
			Owner: "baseowner",
		},
		HeadRepo: models.Repo{
			Name:  "headname",
			Owner: "headowner",
		},
		Pull: models.PullRequest{
			Num:        2,
			HeadBranch: "add-feat",
			BaseBranch: "master",
			Author:     "acme",
		},
		User: models.User{
			Username: "acme-user",
		},
		Log:              logging.NewNoopLogger(),
		Workspace:        "myworkspace",
		RepoRelDir:       "mydir",
		TerraformVersion: projVersion,
	}
	for _, c := range cases {
		t.Run(c.Command, func(t *testing.T) {
			tmpDir, cleanup := TempDir(t)
			defer cleanup()
			var split []string
			if c.Command != "" {
				split = strings.Split(c.Command, " ")
			}
			out, err := r.Run(ctx, split, tmpDir)
			if c.ExpErr != "" {
				ErrContains(t, c.ExpErr, err)
				return
			}
			Ok(t, err)
			// Replace $DIR in the exp with the actual temp dir. We do this
			// here because when constructing the cases we don't yet know the
			// temp dir.
			expOut := strings.Replace(c.ExpOut, "$DIR", tmpDir, -1)
			Equals(t, expOut, out)
		})
	}
}
