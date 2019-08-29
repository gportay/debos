/*
Pacstrap Action

Construct the target rootfs with pacstrap tool.

Yaml syntax:
 - action: pacstrap
   repositories: <list of repositories>
   packages: <list of packages>

Mandatory properties:

- repositories -- list of repositories to use for packages selection.
Properties for repositories are described below.

Yaml syntax for repositories:

 repositories:
   - name: repository name
     server: server url

Optional properties:
- packages -- list of packages to install

Yaml syntax for packages:

 packages:
   - package name
   - package name
*/
package actions

import (
	"fmt"
	"os"
	"path"

	"github.com/go-debos/debos"
)

const configOptionSection = `
[options]
RootDir  = %[1]s
CacheDir = %[1]s/var/cache/pacman/pkg/
GPGDir   = %[1]s/etc/pacman.d/gnupg/
HookDir  = %[1]s/etc/pacman.d/hooks/
HoldPkg  = pacman glibc
Architecture = auto
Color
CheckSpace
SigLevel = Required DatabaseOptional TrustAll
`

const configRepoSection = `

[%s]
Server = %s
`

type Repository struct {
	Name   string
	Server string
}

type PacstrapAction struct {
	debos.BaseAction `yaml:",inline"`
	Repositories []Repository
	Packages     []string
	Config       string `yaml:"config"`
}

func (d *PacstrapAction) Run(context *debos.DebosContext) error {
	d.LogStart()

	// Create config for pacstrap
	configPath := path.Join(context.Scratchdir, "pacman.conf")
	f, err := os.OpenFile(configPath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("Couldn't open pacman config: %v", err)
	}
	_, err = f.WriteString(fmt.Sprintf(configOptionSection, context.Rootdir))
	if err != nil {
		return fmt.Errorf("Couldn't write pacman config: %v", err)
	}
	for _, r := range d.Repositories {
		_, err = f.WriteString(fmt.Sprintf(configRepoSection, r.Name, r.Server))
		if err != nil {
			return fmt.Errorf("Couldn't write to pacman config: %v", err)
		}
	}
	f.Close()

	// Create base layout for pacman-key
	err = os.MkdirAll(path.Join(context.Rootdir, "var", "lib", "pacman"), 0755)
	if err != nil {
		return fmt.Errorf("Couldn't create var/lib/pacman in image: %v", err)
	}
	err = os.MkdirAll(path.Join(context.Rootdir, "etc", "pacman.d", "gnupg"), 0755)
	if err != nil {
		return fmt.Errorf("Couldn't create etc/pacman.d/gnupg in image: %v", err)
	}

	// Copy pacman.conf file
	if len(d.Config) > 0 {
		err = debos.CopyFile(path.Join(context.RecipeDir, d.Config), path.Join(context.Scratchdir, "pacman.conf"), 0644)
		if err != nil {
			return fmt.Errorf("Couldn't copy pacman config: %v", err)
		}
	}

	// Run pacman-key
	cmdline := []string{"pacman-key", "--nocolor", "--config", configPath, "--init"}
	err = debos.Command{}.Run("Pacman-key", cmdline...)
	if err != nil {
		return fmt.Errorf("Couldn't init pacman keyring: %v", err)
	}

	cmdline = []string{"pacman-key", "--nocolor", "--config", configPath, "--populate"}
	err = debos.Command{}.Run("Pacman-key", cmdline...)
	if err != nil {
		return fmt.Errorf("Couldn't populate pacman keyring: %v", err)
	}

	// Run pacstrap
	cmdline = []string{"pacstrap", "-GM", "-C", configPath, context.Rootdir}
	if len(d.Packages) != 0 {
		cmdline = append(cmdline, d.Packages...)
	}
	err = debos.Command{}.Run("Pacstrap", cmdline...)
	if err != nil {
		log := path.Join(context.Rootdir, "var/log/pacman.log")
		_ = debos.Command{}.Run("pacstrap.log", "cat", log)
		return err
	}

	// Remove pacstrap config
	os.Remove(configPath)

	return nil
}
