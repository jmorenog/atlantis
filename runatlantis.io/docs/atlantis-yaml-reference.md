# atlantis.yaml Reference
[[toc]]

::: tip Do I need an atlantis.yaml file?
`atlantis.yaml` files are only required if you wish to customize some aspect of Atlantis.
:::

::: tip Where are the example use cases?
See [www.runatlantis.io/guide/atlantis-yaml-use-cases.html](../guide/atlantis-yaml-use-cases.html)
:::

## Enabling atlantis.yaml
By default all repos are allowed to have an `atlantis.yaml` file, but not all of the keys are enabled by default due to
the sensitive nature of some keys.

Restricted keys can be set in the server side `repos.yaml` file, and you can enable `atlantis.yaml` to override restricted
keys by setting `allowed_overrides` in the `repos.yaml`.  See the [repos.yaml reference](repos-yaml-reference.html) for
more information.

## Example Using All Keys
```yaml
version: 2
automerge: true
projects:
- name: my-project-name
  dir: .
  workspace: default
  terraform_version: v0.11.0
  autoplan:
    when_modified: ["*.tf", "../modules/**.tf"]
    enabled: true
  apply_requirements: [mergeable, approved]
  workflow: myworkflow
workflows:
  myworkflow:
    plan:
      steps:
      - run: my-custom-command arg1 arg2
      - init
      - plan:
          extra_args: ["-lock", "false"]
      - run: my-custom-command arg1 arg2
    apply:
      steps:
      - run: echo hi
      - apply
```

## Usage Notes
* `atlantis.yaml` files must be placed at the root of the repo
* The only supported name is `atlantis.yaml`. Not `atlantis.yml` or `.atlantis.yaml`.
* Once an `atlantis.yaml` file exists in a repo, Atlantis won't try to determine
where to run plan automatically. Instead it will just follow the configuration.
This means that you'll need to define each project in your repo.
* Atlantis uses the `atlantis.yaml` version from the pull request.

## Security
`atlantis.yaml` files allow users to run arbitrary code on the Atlantis server.
This is obviously extremely powerful and dangerous since the Atlantis server will
likely hold your highest privilege credentials.

The risk is increased because Atlantis uses the `atlantis.yaml` file from the
pull request so anyone that can submit a pull request can submit a malicious file.

By default, the keys that are sensitive in nature are restricted from being used in the `atlantis.yaml` file.
Restricted keys can be set in the server side `repos.yaml` file, and you can enable `atlantis.yaml` to override restricted
keys by setting `allowed_overrides` in the `repos.yaml`.  See the [repos.yaml reference](repos-yaml-reference.html) for
more information.

## Reference
### Top-Level Keys
```yaml
version:
automerge:
projects:
workflows:
```
| Key                           | Type                                                             | Default | Required | Description                                                 |
| ----------------------------- | ---------------------------------------------------------------- | ------- | -------- | ----------------------------------------------------------- |
| version                       | int                                                              | none    | yes      | This key is required and must be set to `2`                 |
| automerge                     | bool                                                             | false   | no       | Automatically merge pull request when all plans are applied |
| projects                      | array[[Project](atlantis-yaml-reference.html#project)]           | []      | no       | Lists the projects in this repo                             |
| workflows<br />*(restricted)* | map[string -> [Workflow](atlantis-yaml-reference.html#workflow)] | {}      | no       | Custom workflows                                            |

### Project
```yaml
name: myname
dir: mydir
workspace: myworkspace
autoplan:
terraform_version: 0.11.0
apply_requirements: ["approved"]
workflow: myworkflow
```

| Key                                    | Type                                              | Default | Required | Description                                                                                                                                                                                                           |
| -------------------------------------- | ------------------------------------------------- | ------- | -------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| name                                   | string                                            | none    | maybe    | Required if there is more than one project with the same `dir` and `workspace`. This project name can be used with the `-p` flag.                                                                                     |
| dir                                    | string                                            | none    | yes      | The directory of this project relative to the repo root. Use `.` for the root. For example if the project was under `./project1` then use `project1`                                                                  |
| workspace                              | string                                            | default | no       | The [Terraform workspace](https://www.terraform.io/docs/state/workspaces.html) for this project. Atlantis will switch to this workplace when planning/applying and will create it if it doesn't exist.                |
| autoplan                               | [Autoplan](atlantis-yaml-reference.html#autoplan) | none    | no       | A custom autoplan configuration. If not specified, will use the default algorithm. See [Autoplanning](autoplanning.html).                                                                                             |
| terraform_version                      | string                                            | none    | no       | A specific Terraform version to use when running commands for this project. Must be [Semver compatible](https://semver.org/), ex. `v0.11.0`, `0.12.0-beta1`.                                                          |
| apply_requirements<br />*(restricted)* | array[string]                                     | []      | no       | Requirements that must be satisfied before `atlantis apply` can be run. Currently the only supported requirements are `approved` and `mergeable`. See [Apply Requirements](apply-requirements.html) for more details. |
| workflow <br />*(restricted)*          | string                                            | none    | no       | A custom workflow. If not specified, Atlantis will use its default workflow.                                                                                                                                          |

::: tip
A project represents a Terraform state. Typically, there is one state per directory and workspace however it's possible to
have multiple states in the same directory using `terraform init -backend-config=custom-config.tfvars`.
Atlantis supports this but requires the `name` key to be specified. See [atlantis.yaml Use Cases](../guide/atlantis-yaml-use-cases.html#custom-backend-config) for more details.
:::

### Autoplan
```yaml
enabled: true
when_modified: ["*.tf"]
```
| Key           | Type          | Default | Required | Description                                                                                                                                                                                                                                                                                                              |
| ------------- | ------------- | ------- | -------- |  ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| enabled       | boolean       | true    | no       | Whether autoplanning is enabled for this project.                                                                                                                                                                                                                                                                        |
| when_modified | array[string] | no      | no       | Uses [.dockerignore](https://docs.docker.com/engine/reference/builder/#dockerignore-file) syntax. If any modified file in the pull request matches, this project will be planned. If not specified, Atlantis will use its own algorithm. See [Autoplanning](autoplanning.html). Paths are relative to the project's dir. |

### Workflow
```yaml
plan:
apply:
```

| Key   | Type                                        | Default               | Required |  Description                    |
| ----- | ------------------------------------------- | --------------------- | -------- |  ------------------------------ |
| plan  | [Stage](atlantis-yaml-reference.html#stage) | `steps: [init, plan]` | no       | How to plan for this project.  |
| apply | [Stage](atlantis-yaml-reference.html#stage) | `steps: [apply]`      | no       | How to apply for this project. |

### Stage
```yaml
steps:
- run: custom-command
- init
- plan:
    extra_args: [-lock=false]
```

| Key   | Type                                             | Default | Required | Description                                                                                   |
| ----- | ------------------------------------------------ | ------- | -------- | --------------------------------------------------------------------------------------------- |
| steps | array[[Step](atlantis-yaml-reference.html#step)] | `[]`    | no       | List of steps for this stage. If the steps key is empty, no steps will be run for this stage. |

### Step
#### Built-In Commands: init, plan, apply
Steps can be a single string for a built-in command.
```yaml
- init
- plan
- apply
```
| Key             | Type   | Default | Required | Description                                                                                            |
| --------------- | ------ | ------- | -------- | ------------------------------------------------------------------------------------------------------ |
| init/plan/apply | string | none    | no       | Use a built-in command without additional configuration. Only `init`, `plan` and `apply` are supported |

#### Built-In Command With Extra Args
A map from string to `extra_args` for a built-in command with extra arguments.
```yaml
- init:
    extra_args: [arg1, arg2]
- plan:
    extra_args: [arg1, arg2]
- apply:
    extra_args: [arg1, arg2]
```
| Key             | Type                               | Default | Required | Description                                                                                                                                         |
| --------------- | ---------------------------------- | ------- | -------- | --------------------------------------------------------------------------------------------------------------------------------------------------- |
| init/plan/apply | map[`extra_args` -> array[string]] | none    | no       | Use a built-in command and append `extra_args`. Only `init`, `plan` and `apply` are supported as keys and only `extra_args` is supported as a value |
#### Custom `run` Command
Or a custom command
```yaml
- run: custom-command
```
| Key | Type   | Default | Required | Description          |
| --- | ------ | ------- | -------- | -------------------- |
| run | string | none    | no       | Run a custom command |

::: tip
`run` steps are executed with the following environment variables:
* `WORKSPACE` - The Terraform workspace used for this project, ex. `default`.
  * NOTE: if the step is executed before `init` then Atlantis won't have switched to this workspace yet.
* `ATLANTIS_TERRAFORM_VERSION` - The version of Terraform used for this project, ex. `0.11.0`.
* `DIR` - Absolute path to the current directory.
* `PLANFILE` - Absolute path to the location where Atlantis expects the plan to
either be generated (by plan) or already exist (if running apply). Can be used to
override the built-in `plan`/`apply` commands, ex. `run: terraform plan -out $PLANFILE`.
* `BASE_REPO_NAME` - Name of the repository that the pull request will be merged into, ex. `atlantis`.
* `BASE_REPO_OWNER` - Owner of the repository that the pull request will be merged into, ex. `runatlantis`.
* `HEAD_REPO_NAME` - Name of the repository that is getting merged into the base repository, ex. `atlantis`.
* `HEAD_REPO_OWNER` - Owner of the repository that is getting merged into the base repository, ex. `acme-corp`.
* `HEAD_BRANCH_NAME` - Name of the head branch of the pull request (the branch that is getting merged into the base)
* `BASE_BRANCH_NAME` - Name of the base branch of the pull request (the branch that the pull request is getting merged into)
* `PULL_NUM` - Pull request number or ID, ex. `2`.
* `PULL_AUTHOR` - Username of the pull request author, ex. `acme-user`.
* `USER_NAME` - Username of the VCS user running command, ex. `acme-user`. During an autoplan, the user will be the Atlantis API user, ex. `atlantis`.
:::

::: tip
Note that a custom command will only terminate if all output file descriptors are closed.
Therefore a custom command can only be sent to the background (e.g. for an SSH tunnel during
the terraform run) when its output is redirected to a different location. For example, atlantis
will execute a custom script containing the following code to create a SSH tunnel correctly: 
`ssh -f -M -S /tmp/ssh_tunnel -L 3306:database:3306 -N bastion 1>/dev/null 2>&1`. Without
the pipe, the script would block the atlantis workflow.
:::

## Next Steps
Check out the [atlantis.yaml Use Cases](../guide/atlantis-yaml-use-cases.html) for
some real world examples.
