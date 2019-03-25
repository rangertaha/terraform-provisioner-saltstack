// This package implements a provisioner for Terraform that executes a
// saltstack state within the remote machine and add provider infromation as
// grains to the nodes.
//

package saltstack

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/hashicorp/hcl"
	"github.com/hashicorp/terraform/communicator"
	"github.com/hashicorp/terraform/communicator/remote"
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/hashicorp/terraform/terraform"
	linereader "github.com/mitchellh/go-linereader"
)

type provisionFn func(terraform.UIOutput, communicator.Communicator) error

type provisioner struct {
	SkipBootstrap  bool
	BootstrapArgs  string
	LocalStateTree string
	DisableSudo    bool
	CustomState    string

	MinionConfig string
	//MasterConfig      string
	//MasterHost       string

	LocalPillarRoots  string
	RemoteStateTree   string
	RemotePillarRoots string
	TempConfigDir     string
	NoExitOnFailure   bool
	LogLevel          string
	SaltCallArgs      string
	CmdArgs           string
	Grains            bool
	EnableGrains      bool
	TfVars            string
	SudoPassword	string
	//LocalGrains       []string
	//PreSaltScripts []string
	//PostSaltScripts []string
}

const DefaultStateTreeDir = "/srv/salt"
const DefaultPillarRootDir = "/srv/pillar"
const RemoteGrainsFile = "/etc/salt/grains"

const DefaultStateDir = "/srv/salt"
const DefaultPillarDir = "/srv/pillar"
const DefaultGrainsFile = "/etc/salt/grains"

// Provisioner returns a saltstack provisioner
func Provisioner() terraform.ResourceProvisioner {
	return &schema.Provisioner{
		Schema: map[string]*schema.Schema{
			"local_state_tree": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
			},
			"local_pillar_roots": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},
			"remote_state_tree": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},
			"remote_pillar_roots": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},
			"temp_config_dir": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				Default:  "/tmp/salt",
			},
			"skip_bootstrap": &schema.Schema{
				Type:     schema.TypeBool,
				Optional: true,
			},
			"no_exit_on_failure": &schema.Schema{
				Type:     schema.TypeBool,
				Optional: true,
			},
			"bootstrap_args": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},
			"disable_sudo": &schema.Schema{
				Type:     schema.TypeBool,
				Optional: true,
			},
			"custom_state": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},
			"minion_config_file": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},
			"cmd_args": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},
			"salt_call_args": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},
			"log_level": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},
			"tfvars": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				Default:  "terraform.tfvars",
			},
			"sudo_password": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				Default:  "",
			},
			"grains": &schema.Schema{
				Type:     schema.TypeBool,
				Optional: true,
				Default:  true,
			},
		},

		ApplyFunc:    applyFn,
		ValidateFunc: validateFn,
	}
}

// Apply executes the file provisioner
func applyFn(ctx context.Context) error {
	// Decode the raw config for this provisioner
	o := ctx.Value(schema.ProvOutputKey).(terraform.UIOutput)
	d := ctx.Value(schema.ProvConfigDataKey).(*schema.ResourceData)
	s := ctx.Value(schema.ProvRawStateKey).(*terraform.InstanceState)

	connState := ctx.Value(schema.ProvRawStateKey).(*terraform.InstanceState)

	p, err := decodeConfig(d)
	if err != nil {
		return err
	}

	// Get a new communicator
	comm, err := communicator.New(connState)
	if err != nil {
		return err
	}

	retryCtx, cancel := context.WithTimeout(ctx, comm.Timeout())
	defer cancel()

	// Wait and retry until we establish the connection
	err = communicator.Retry(retryCtx, func() error {
		return comm.Connect(o)
	})

	if err != nil {
		return err
	}

	// Wait for the context to end and then disconnect
	go func() {
		<-ctx.Done()
		comm.Disconnect()
	}()

	var src, dst string

	o.Output("Provisioning with Salt...")
	if !p.SkipBootstrap {
		cmd := &remote.Cmd{
			// Fallback on wget if curl failed for any reason (such as not being installed)
			Command: fmt.Sprintf("curl -L https://bootstrap.saltstack.com -o /tmp/install_salt.sh || wget -O /tmp/install_salt.sh https://bootstrap.saltstack.com"),
		}
		o.Output(fmt.Sprintf("Downloading saltstack bootstrap to /tmp/install_salt.sh"))
		if err = comm.Start(cmd); err != nil {
			err = fmt.Errorf("Unable to download Salt: %s", err)
		}

		if err := cmd.Wait(); err != nil {
			return err
		}

		outR, outW := io.Pipe()
		errR, errW := io.Pipe()
		go copyOutput(o, outR)
		go copyOutput(o, errR)
		defer outW.Close()
		defer errW.Close()

		cmd = &remote.Cmd{
			Command: fmt.Sprintf("%s /tmp/install_salt.sh %s", p.sudo("sh"), p.BootstrapArgs),
			Stdout:  outW,
			Stderr:  errW,
		}

		o.Output(fmt.Sprintf("Installing Salt with command %s", cmd.Command))
		if err := comm.Start(cmd); err != nil {
			return fmt.Errorf("Unable to install Salt: %s", err)
		}

		if err := cmd.Wait(); err != nil {
			return err
		}
	}

	o.Output(fmt.Sprintf("Creating remote temporary directory: %s", p.TempConfigDir))
	if err := p.createDir(o, comm, p.TempConfigDir); err != nil {
		return fmt.Errorf("Error creating remote temporary directory: %s", err)
	}

	if p.Grains {
		o.Output(fmt.Sprintf("Uploading grain file"))

		// Get the source
		src, deleteSource, err := getGrains(d, s, o)
		if err != nil {
			return err
		}
		if deleteSource {
			defer os.Remove(src)
		}

		dst := filepath.ToSlash(filepath.Join(p.TempConfigDir, "grains"))
		if err = p.uploadFile(o, comm, dst, src); err != nil {
			return fmt.Errorf("Error uploading local grains config file to remote: %s", err)
		}

		src = filepath.ToSlash(filepath.Join(p.TempConfigDir, "grains"))
		dst = "/etc/salt/grains"
		if err = p.moveFile(o, comm, dst, src); err != nil {
			return fmt.Errorf("Unable to move %s/grains to %s: %s", p.TempConfigDir, dst, err)
		}
	}

	if p.MinionConfig != "" {
		o.Output(fmt.Sprintf("Uploading minion config: %s", p.MinionConfig))
		src = p.MinionConfig
		dst = filepath.ToSlash(filepath.Join(p.TempConfigDir, "minion"))
		if err = p.uploadFile(o, comm, dst, src); err != nil {
			return fmt.Errorf("Error uploading local minion config file to remote: %s", err)
		}

		// move minion config into /etc/salt
		o.Output(fmt.Sprintf("Make sure directory %s exists", "/etc/salt"))
		if err := p.createDir(o, comm, "/etc/salt"); err != nil {
			return fmt.Errorf("Error creating remote salt configuration directory: %s", err)
		}
		src = filepath.ToSlash(filepath.Join(p.TempConfigDir, "minion"))
		dst = "/etc/salt/minion"
		if err = p.moveFile(o, comm, dst, src); err != nil {
			return fmt.Errorf("Unable to move %s/minion to /etc/salt/minion: %s", p.TempConfigDir, err)
		}
	}

	o.Output(fmt.Sprintf("Uploading local state tree: %s", p.LocalStateTree))
	src = p.LocalStateTree
	dst = filepath.ToSlash(filepath.Join(p.TempConfigDir, "states"))
	if err = p.uploadDir(o, comm, dst, src, []string{".git"}); err != nil {
		return fmt.Errorf("Error uploading local state tree to remote: %s", err)
	}

	// move state tree from temporary directory
	src = filepath.ToSlash(filepath.Join(p.TempConfigDir, "states"))
	dst = p.RemoteStateTree
	if err = p.removeDir(o, comm, dst); err != nil {
		return fmt.Errorf("Unable to clear salt tree: %s", err)
	}
	if err = p.moveFile(o, comm, dst, src); err != nil {
		return fmt.Errorf("Unable to move %s/states to %s: %s", p.TempConfigDir, dst, err)
	}

	if p.LocalPillarRoots != "" {
		o.Output(fmt.Sprintf("Uploading local pillar roots: %s", p.LocalPillarRoots))
		src = p.LocalPillarRoots
		dst = filepath.ToSlash(filepath.Join(p.TempConfigDir, "pillar"))
		if err = p.uploadDir(o, comm, dst, src, []string{".git"}); err != nil {
			return fmt.Errorf("Error uploading local pillar roots to remote: %s", err)
		}

		// move pillar root from temporary directory
		src = filepath.ToSlash(filepath.Join(p.TempConfigDir, "pillar"))
		dst = p.RemotePillarRoots

		if err = p.removeDir(o, comm, dst); err != nil {
			return fmt.Errorf("Unable to clear pillar root: %s", err)
		}
		if err = p.moveFile(o, comm, dst, src); err != nil {
			return fmt.Errorf("Unable to move %s/pillar to %s: %s", p.TempConfigDir, dst, err)
		}
	}

	outR, outW := io.Pipe()
	errR, errW := io.Pipe()
	go copyOutput(o, outR)
	go copyOutput(o, errR)
	defer outW.Close()
	defer errW.Close()

	o.Output(fmt.Sprintf("Running: salt-call --local %s", p.CmdArgs))
	cmd := &remote.Cmd{
		Command: p.sudo(fmt.Sprintf("salt-call --local %s", p.CmdArgs)),
		Stdout:  outW,
		Stderr:  errW,
	}
	if err = comm.Start(cmd); err != nil {
		err = fmt.Errorf("Error executing salt-call: %s", err)
	}

	if err := cmd.Wait(); err != nil {
		return err
	}
	return nil
}

// getGrains returns the file to use as the source
func getGrains(data *schema.ResourceData, state *terraform.InstanceState, ui terraform.UIOutput) (string, bool, error) {

	jsonGrain, err := json.Marshal(&state)
	if err != nil {
		return "", false, err
	}
	ui.Output(fmt.Sprintf("Provider state grains: %s", jsonGrain))

	var providerGrains map[string]interface{}
	if err := hcl.Decode(&providerGrains, string(jsonGrain)); err != nil {
		return "", false, fmt.Errorf("Error decoding json grains: %s", err)
	}
	ui.Output(fmt.Sprintf("Decoded provider state grains: %s", providerGrains))

	grains := make(map[string]interface{})
	for k, v := range providerGrains {
		if _, ok := providerGrains[k]; ok {
			grains[k] = v
		}
	}
	ui.Output(fmt.Sprintf("Merged privider grains with all grains: %s", grains))

	dir, err := os.Getwd()
	if err != nil {
		return "", false, err
	}
	ui.Output(fmt.Sprintf("Looking for terraform.ftvars in directory: %s", dir))

	tfvarsFile := data.Get("tfvars").(string)
	input, err := ioutil.ReadFile(tfvarsFile)
	if err != nil {
		return "", false, err
	}

	ui.Output(fmt.Sprintf("Decoding terraform.ftvars file for grains: %s", input))
	var parsed map[string]interface{}
	if err := hcl.Decode(&parsed, string(input)); err != nil {
		return "", false, fmt.Errorf("Error decoding terraform.tfvars file for grains: %s", err)
	}

	for k, v := range parsed {
		if _, ok := parsed[k]; ok {
			grains[k] = v
		}
	}
	ui.Output(fmt.Sprintf("Merged privider grains with all grains: %s", grains))

	jsonGrain, err = json.Marshal(&grains)
	if err != nil {
		return "", false, err
	}
	ui.Output(fmt.Sprintf("Converted grains to json format: %s", jsonGrain))

	file, err := ioutil.TempFile("", "tf-grain-content")
	if err != nil {
		return "", true, err
	}
	content := string(jsonGrain)
	if _, err = file.WriteString(content); err != nil {
		return "", true, err
	}

	return file.Name(), true, nil
}

// Prepends sudo to supplied command if config says to
func (p *provisioner) sudo(cmd string) string {
	if p.DisableSudo {
		return cmd
	}

	if p.SudoPassword != "" {
		return "echo '" + p.SudoPassword + "' | sudo -S " + cmd
	}

	return "sudo " + cmd
}

func validateDirConfig(path string, name string, required bool) error {
	if required == true && path == "" {
		return fmt.Errorf("%s cannot be empty", name)
	} else if required == false && path == "" {
		return nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("%s: path '%s' is invalid: %s", name, path, err)
	} else if !info.IsDir() {
		return fmt.Errorf("%s: path '%s' must point to a directory", name, path)
	}
	return nil
}

func validateFileConfig(path string, name string, required bool) error {
	if required == true && path == "" {
		return fmt.Errorf("%s cannot be empty", name)
	} else if required == false && path == "" {
		return nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("%s: path '%s' is invalid: %s", name, path, err)
	} else if info.IsDir() {
		return fmt.Errorf("%s: path '%s' must point to a file", name, path)
	}
	return nil
}

func (p *provisioner) uploadFile(o terraform.UIOutput, comm communicator.Communicator, dst, src string) error {
	f, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("Error opening: %s", err)
	}
	defer f.Close()

	if err = comm.Upload(dst, f); err != nil {
		return fmt.Errorf("Error uploading %s: %s", src, err)
	}
	return nil
}

func (p *provisioner) moveFile(o terraform.UIOutput, comm communicator.Communicator, dst, src string) error {
	o.Output(fmt.Sprintf("Moving %s to %s", src, dst))
	cmd := &remote.Cmd{Command: fmt.Sprintf(p.sudo("mv %s %s"), src, dst)}
	if err := comm.Start(cmd); err != nil {
		return fmt.Errorf("Unable to move %s to %s: %s", src, dst, err)
	}
	if err := cmd.Wait(); err != nil {
		return err
	}
	return nil
}

func (p *provisioner) createDir(o terraform.UIOutput, comm communicator.Communicator, dir string) error {
	o.Output(fmt.Sprintf("Creating directory: %s", dir))
	cmd := &remote.Cmd{
		Command: fmt.Sprintf("mkdir -p '%s'", dir),
	}
	if err := comm.Start(cmd); err != nil {
		return err
	}

	if err := cmd.Wait(); err != nil {
		return err
	}
	return nil
}

func (p *provisioner) removeDir(o terraform.UIOutput, comm communicator.Communicator, dir string) error {
	o.Output(fmt.Sprintf("Removing directory: %s", dir))
	cmd := &remote.Cmd{
		Command: fmt.Sprintf("rm -rf '%s'", dir),
	}
	if err := comm.Start(cmd); err != nil {
		return err
	}
	if err := cmd.Wait(); err != nil {
		return err
	}
	return nil
}

func (p *provisioner) uploadDir(o terraform.UIOutput, comm communicator.Communicator, dst, src string, ignore []string) error {
	if err := p.createDir(o, comm, dst); err != nil {
		return err
	}

	// Make sure there is a trailing "/" so that the directory isn't
	// created on the other side.
	if src[len(src)-1] != '/' {
		src = src + "/"
	}
	return comm.UploadDir(dst, src)
}

// func (p *provisioner) removeSalt(o terraform.UIOutput, comm communicator.Communicator) error {
// 	o.Output(fmt.Sprintf("Removing SaltStack"))
// 	cmd := &remote.Cmd{
// 		Command: fmt.Sprintf("rm -rf '%s'", dir),
// 	}
// 	if err := comm.Start(cmd); err != nil {
// 		return err
// 	}
// 	if err := cmd.Wait(); err != nil {
// 		return err
// 	}
// 	return nil
// }

// Validate checks if the required arguments are configured
func validateFn(c *terraform.ResourceConfig) (ws []string, es []error) {
	// require a salt state tree
	localStateTreeTmp, ok := c.Get("local_state_tree")
	var localStateTree string
	if !ok {
		es = append(es,
			errors.New("Required local_state_tree is not set"))
	} else {
		localStateTree = localStateTreeTmp.(string)
	}
	err := validateDirConfig(localStateTree, "local_state_tree", true)
	if err != nil {
		es = append(es, err)
	}

	var localPillarRoots string
	localPillarRootsTmp, ok := c.Get("local_pillar_roots")
	if !ok {
		localPillarRoots = ""
	} else {
		localPillarRoots = localPillarRootsTmp.(string)
	}

	err = validateDirConfig(localPillarRoots, "local_pillar_roots", false)
	if err != nil {
		es = append(es, err)
	}

	var minionConfig string
	minionConfigTmp, ok := c.Get("minion_config_file")
	if !ok {
		minionConfig = ""
	} else {
		minionConfig = minionConfigTmp.(string)
	}
	err = validateFileConfig(minionConfig, "minion_config_file", false)
	if err != nil {
		es = append(es, err)
	}

	var remoteStateTree string
	remoteStateTreeTmp, ok := c.Get("remote_state_tree")
	if !ok {
		remoteStateTree = ""
	} else {
		remoteStateTree = remoteStateTreeTmp.(string)
	}

	var remotePillarRoots string
	remotePillarRootsTmp, ok := c.Get("remote_pillar_roots")
	if !ok {
		remotePillarRoots = ""
	} else {
		remotePillarRoots = remotePillarRootsTmp.(string)
	}

	if minionConfig != "" && (remoteStateTree != "" || remotePillarRoots != "") {
		es = append(es,
			errors.New("remote_state_tree and remote_pillar_roots only apply when minion_config_file is not used"))
	}

	if len(es) > 0 {
		return ws, es
	}

	return ws, es
}

func decodeConfig(d *schema.ResourceData) (*provisioner, error) {
	p := &provisioner{
		LocalStateTree:    d.Get("local_state_tree").(string),
		LogLevel:          d.Get("log_level").(string),
		SaltCallArgs:      d.Get("salt_call_args").(string),
		CmdArgs:           d.Get("cmd_args").(string),
		MinionConfig:      d.Get("minion_config_file").(string),
		CustomState:       d.Get("custom_state").(string),
		DisableSudo:       d.Get("disable_sudo").(bool),
		BootstrapArgs:     d.Get("bootstrap_args").(string),
		NoExitOnFailure:   d.Get("no_exit_on_failure").(bool),
		SkipBootstrap:     d.Get("skip_bootstrap").(bool),
		TempConfigDir:     d.Get("temp_config_dir").(string),
		RemotePillarRoots: d.Get("remote_pillar_roots").(string),
		RemoteStateTree:   d.Get("remote_state_tree").(string),
		LocalPillarRoots:  d.Get("local_pillar_roots").(string),
		Grains:            d.Get("grains").(bool),
		TfVars:            d.Get("tfvars").(string),
		SudoPassword:            d.Get("sudo_password").(string),
	}

	// build the command line args to pass onto salt
	var cmdArgs bytes.Buffer

	if p.CustomState == "" {
		cmdArgs.WriteString(" state.highstate")
	} else {
		cmdArgs.WriteString(" state.sls ")
		cmdArgs.WriteString(p.CustomState)
	}

	if p.MinionConfig == "" {
		// pass --file-root and --pillar-root if no minion_config_file is supplied
		if p.RemoteStateTree != "" {
			cmdArgs.WriteString(" --file-root=")
			cmdArgs.WriteString(p.RemoteStateTree)
		} else {
			cmdArgs.WriteString(" --file-root=")
			cmdArgs.WriteString(DefaultStateTreeDir)
		}
		if p.RemotePillarRoots != "" {
			cmdArgs.WriteString(" --pillar-root=")
			cmdArgs.WriteString(p.RemotePillarRoots)
		} else {
			cmdArgs.WriteString(" --pillar-root=")
			cmdArgs.WriteString(DefaultPillarRootDir)
		}
	}

	if !p.NoExitOnFailure {
		cmdArgs.WriteString(" --retcode-passthrough")
	}

	if p.LogLevel == "" {
		cmdArgs.WriteString(" -l info")
	} else {
		cmdArgs.WriteString(" -l ")
		cmdArgs.WriteString(p.LogLevel)
	}

	if p.SaltCallArgs != "" {
		cmdArgs.WriteString(" ")
		cmdArgs.WriteString(p.SaltCallArgs)
	}

	p.CmdArgs = cmdArgs.String()

	return p, nil
}

func copyOutput(
	o terraform.UIOutput, r io.Reader) {
	lr := linereader.New(r)
	for line := range lr.Ch {
		o.Output(line)
	}
}
