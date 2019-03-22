package raw

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/go-ozzo/ozzo-validation"
	"github.com/hashicorp/go-version"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/events/yaml/valid"
)

const (
	DefaultWorkspace          = "default"
	ApprovedApplyRequirement  = "approved"
	MergeableApplyRequirement = "mergeable"
)

type Project struct {
	Name              *string   `yaml:"name,omitempty"`
	Dir               *string   `yaml:"dir,omitempty"`
	Workspace         *string   `yaml:"workspace,omitempty"`
	Workflow          *string   `yaml:"workflow,omitempty"`
	TerraformVersion  *string   `yaml:"terraform_version,omitempty"`
	Autoplan          *Autoplan `yaml:"autoplan,omitempty"`
	ApplyRequirements []string  `yaml:"apply_requirements,omitempty"`
}

func (p Project) Validate() error {
	hasDotDot := func(value interface{}) error {
		if strings.Contains(*value.(*string), "..") {
			return errors.New("cannot contain '..'")
		}
		return nil
	}

	validTFVersion := func(value interface{}) error {
		strPtr := value.(*string)
		if strPtr == nil {
			return nil
		}
		_, err := version.NewVersion(*strPtr)
		return errors.Wrapf(err, "version %q could not be parsed", *strPtr)
	}
	validName := func(value interface{}) error {
		strPtr := value.(*string)
		if strPtr == nil {
			return nil
		}
		if *strPtr == "" {
			return errors.New("if set cannot be empty")
		}
		if !validProjectName(*strPtr) {
			return fmt.Errorf("%q is not allowed: must contain only URL safe characters", *strPtr)
		}
		return nil
	}
	return validation.ValidateStruct(&p,
		validation.Field(&p.Dir, validation.Required, validation.By(hasDotDot)),
		validation.Field(&p.ApplyRequirements, validation.By(validApplyReq)),
		validation.Field(&p.TerraformVersion, validation.By(validTFVersion)),
		validation.Field(&p.Name, validation.By(validName)),
	)
}

func (p Project) ToValid() valid.Project {
	var v valid.Project
	cleanedDir := filepath.Clean(*p.Dir)
	if cleanedDir == "/" {
		cleanedDir = "."
	}
	v.Dir = cleanedDir

	if p.Workspace == nil || *p.Workspace == "" {
		v.Workspace = DefaultWorkspace
	} else {
		v.Workspace = *p.Workspace
	}

	v.Workflow = p.Workflow
	if p.TerraformVersion != nil {
		v.TerraformVersion, _ = version.NewVersion(*p.TerraformVersion)
	}
	if p.Autoplan == nil {
		v.Autoplan = DefaultAutoPlan()
	} else {
		v.Autoplan = p.Autoplan.ToValid()
	}

	// There are no default apply requirements.
	v.ApplyRequirements = p.ApplyRequirements

	v.Name = p.Name

	return v
}

// validProjectName returns true if the project name is valid.
// Since the name might be used in URLs and definitely in files we don't
// support any characters that must be url escaped *except* for '/' because
// users like to name their projects to match the directory it's in.
func validProjectName(name string) bool {
	nameWithoutSlashes := strings.Replace(name, "/", "-", -1)
	return nameWithoutSlashes == url.QueryEscape(nameWithoutSlashes)
}

func validApplyReq(value interface{}) error {
	reqs := value.([]string)
	for _, r := range reqs {
		if r != ApprovedApplyRequirement && r != MergeableApplyRequirement {
			return fmt.Errorf("%q not supported, only %s and %s are supported", r, ApprovedApplyRequirement, MergeableApplyRequirement)
		}
	}
	return nil
}
