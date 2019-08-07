package apply

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"

	"github.com/GoogleCloudPlatform/healthcare/deploy/config"
	"github.com/GoogleCloudPlatform/healthcare/deploy/terraform"
)

// Standard (built in) roles required by the Forseti service account on projects to be monitored.
// This list includes project level roles from
// https://github.com/forseti-security/terraform-google-forseti/blob/master/modules/server/main.tf#L63
// In the future, have a deeper integration with Forseti module and reuse the role list.
var forsetiStandardRoles = [...]string{
	"appengine.appViewer",
	"bigquery.metadataViewer",
	"browser",
	"cloudasset.viewer",
	"cloudsql.viewer",
	"compute.networkViewer",
	"iam.securityReviewer",
	"servicemanagement.quotaViewer",
	"serviceusage.serviceUsageConsumer",
}

// Forseti applies project configuration to a Forseti project.
func Forseti(conf *config.Config, project *config.Project, opts *Options) error {
	if err := Default(conf, project, opts); err != nil {
		return err
	}
	if err := ForsetiConfig(conf); err != nil {
		return fmt.Errorf("failed to apply forseti config for forseti project %q: %v", conf.Forseti.Project.ID)
	}

	// TODO: write service account and bucket to generated fields.

	return nil
}

// ForsetiConfig applies the forseti config, if it exists. It does not configure
// other settings such as billing account, deletion lien, etc.
// TODO Make it private or merge it into Forseti() after removing apply_forseti.go.
func ForsetiConfig(conf *config.Config) error {
	if conf.Forseti == nil {
		log.Println("no forseti config, nothing to do")
		return nil
	}

	// TODO: use registry instead of cloning repo
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	runCmd := func(bin string, args ...string) error {
		cmd := exec.Command(bin, args...)
		cmd.Dir = dir
		return cmdRun(cmd)
	}
	if err := runCmd("git", "clone", "https://github.com/forseti-security/terraform-google-forseti"); err != nil {
		return fmt.Errorf("failed to clone forseti module repo: %v", err)
	}

	tfConf := &terraform.Config{
		Modules: map[string]*terraform.Module{
			"forseti": &terraform.Module{
				Source:     "./terraform-google-forseti",
				Properties: conf.Forseti.Properties,
			},
		},
	}
	return terraformApply(tfConf, dir)
}

// GrantForsetiPermissions grants all necessary permissions to the given Forseti service account in the project.
// TODO Use Terraform to deploy these.
func GrantForsetiPermissions(projectID, serviceAccount string) error {
	for _, r := range forsetiStandardRoles {
		if err := addBinding(projectID, serviceAccount, r); err != nil {
			return fmt.Errorf("failed to grant all necessary permissions to Forseti service account %q in project %q: %v", serviceAccount, projectID, err)
		}
	}
	return nil
}
