package yaml

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/go-ozzo/ozzo-validation"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/events/yaml/raw"
	"github.com/runatlantis/atlantis/server/events/yaml/valid"
	"gopkg.in/yaml.v2"
)

// AtlantisYAMLFilename is the name of the config file for each repo.
const AtlantisYAMLFilename = "atlantis.yaml"

type ParserValidator struct{}

// ReadConfig returns the parsed and validated atlantis.yaml config for repoDir.
// If there was no config file, then this can be detected by checking the type
// of error: os.IsNotExist(error) but it's instead preferred to check with
// HasRepoCfg.
func (p *ParserValidator) ReadConfig(repoDir string) (valid.RepoCfg, error) {
	configFile := p.configFilePath(repoDir)
	configData, err := ioutil.ReadFile(configFile) // nolint: gosec

	// NOTE: the error we return here must also be os.IsNotExist since that's
	// what our callers use to detect a missing config file.
	if err != nil && os.IsNotExist(err) {
		return valid.RepoCfg{}, err
	}

	// If it exists but we couldn't read it return an error.
	if err != nil {
		return valid.RepoCfg{}, errors.Wrapf(err, "unable to read %s file", AtlantisYAMLFilename)
	}

	// If the config file exists, parse it.
	config, err := p.parseAndValidate(configData)
	if err != nil {
		return valid.RepoCfg{}, errors.Wrapf(err, "parsing %s", AtlantisYAMLFilename)
	}
	return config, err
}

func (p *ParserValidator) HasRepoCfg(repoDir string) (bool, error) {
	_, err := os.Stat(p.configFilePath(repoDir))
	if os.IsNotExist(err) {
		return false, nil
	}
	if err == nil {
		return true, nil
	}
	return false, err
}

func (p *ParserValidator) configFilePath(repoDir string) string {
	return filepath.Join(repoDir, AtlantisYAMLFilename)
}

func (p *ParserValidator) parseAndValidate(configData []byte) (valid.RepoCfg, error) {
	var rawConfig raw.Config
	if err := yaml.UnmarshalStrict(configData, &rawConfig); err != nil {
		return valid.RepoCfg{}, err
	}

	// Set ErrorTag to yaml so it uses the YAML field names in error messages.
	validation.ErrorTag = "yaml"

	if err := rawConfig.Validate(); err != nil {
		return valid.RepoCfg{}, err
	}

	// Top level validation.
	if err := p.validateWorkflows(rawConfig); err != nil {
		return valid.RepoCfg{}, err
	}

	validConfig := rawConfig.ToValid()
	if err := p.validateProjectNames(validConfig); err != nil {
		return valid.RepoCfg{}, err
	}

	return validConfig, nil
}

func (p *ParserValidator) validateProjectNames(config valid.RepoCfg) error {
	// First, validate that all names are unique.
	seen := make(map[string]bool)
	for _, project := range config.Projects {
		if project.Name != nil {
			name := *project.Name
			exists := seen[name]
			if exists {
				return fmt.Errorf("found two or more projects with name %q; project names must be unique", name)
			}
			seen[name] = true
		}
	}

	// Next, validate that all dir/workspace combos are named.
	// This map's keys will be 'dir/workspace' and the values are the names for
	// that project.
	dirWorkspaceToNames := make(map[string][]string)
	for _, project := range config.Projects {
		key := fmt.Sprintf("%s/%s", project.Dir, project.Workspace)
		names := dirWorkspaceToNames[key]

		// If there is already a project with this dir/workspace then this
		// project must have a name.
		if len(names) > 0 && project.Name == nil {
			return fmt.Errorf("there are two or more projects with dir: %q workspace: %q that are not all named; they must have a 'name' key so they can be targeted for apply's separately", project.Dir, project.Workspace)
		}
		var name string
		if project.Name != nil {
			name = *project.Name
		}
		dirWorkspaceToNames[key] = append(dirWorkspaceToNames[key], name)
	}

	return nil
}

func (p *ParserValidator) validateWorkflows(config raw.Config) error {
	for _, project := range config.Projects {
		if err := p.validateWorkflowExists(project, config.Workflows); err != nil {
			return err
		}
	}
	return nil
}

func (p *ParserValidator) validateWorkflowExists(project raw.Project, workflows map[string]raw.Workflow) error {
	if project.Workflow == nil {
		return nil
	}
	workflow := *project.Workflow
	for k := range workflows {
		if k == workflow {
			return nil
		}
	}
	return fmt.Errorf("workflow %q is not defined", workflow)
}
