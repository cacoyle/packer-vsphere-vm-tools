package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	//"github.com/davecgh/go-spew/spew"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25/mo"
	vmwcommon "github.com/hashicorp/packer/builder/vmware/common"
	"github.com/hashicorp/packer/common"
	"github.com/hashicorp/packer/packer"
	"github.com/hashicorp/packer/helper/config"
	"github.com/hashicorp/packer/post-processor/vsphere"
	"github.com/hashicorp/packer/template/interpolate"
)

type Config struct {
	common.PackerConfig `mapstructure:",squash"`
	Host		    string `mapstructure:"host"`
	Insecure	    bool   `mapstructure:"insecure"`
	Username	    string `mapstructure:"username"`
	Password	    string `mapstructure:"password"`

	ctx interpolate.Context
}

type PostProcessor struct {
	config	Config
	url	*url.URL
}

var builtins = map[string]string{
	vsphere.BuilderId:	"vmware",
	vmwcommon.BuilderIdESX: "vmware",
}

func (p *PostProcessor) Configure (raws ...interface{}) error {
	err := config.Decode(&p.config, &config.DecodeOpts{
		Interpolate:		true,
		InterpolateContext:	&p.config.ctx,
		InterpolateFilter:	&interpolate.RenderFilter{
			Exclude: []string{},
		},
	}, raws...)

	if err != nil {
		return err
	}

	errs := new(packer.MultiError)

	vc := map[string]*string{
		"host":		&p.config.Host,
		"username":	&p.config.Username,
		"password":	&p.config.Password,
	}

	for key, ptr := range vc {
		if *ptr == "" {
			errs = packer.MultiErrorAppend(
				errs, fmt.Errorf("%s must be set", key))
		}
	}

	sdk, err := url.Parse(fmt.Sprintf("https://%v/sdk", p.config.Host))
	if err != nil {
		errs = packer.MultiErrorAppend(
			errs, fmt.Errorf("Error invalid vSphere sdk endpoint: %s", err))
		return errs
	}

	sdk.User = url.UserPassword(p.config.Username, p.config.Password)
	p.url = sdk

	if len(errs.Errors) > 0 {
		return errs
	}

	return nil
}

func (p *PostProcessor) PostProcess(ctx context.Context, ui packer.Ui, artifact packer.Artifact) (packer.Artifact, bool, bool, error) {
	if _, ok := builtins[artifact.BuilderId()]; !ok {
		return nil, false, false, fmt.Errorf("The Packer vSphere Template post-processor "+
			"can only take an artifact from the VMware-iso builder, built on "+
			"ESXi (i.e. remote) or an artifact from the vSphere post-processor. "+
			"Artifact type %s does not fit this requirement", artifact.BuilderId())
	}
	log.Printf("Incoming builder id: %s", artifact.BuilderId())
	f := artifact.State(vmwcommon.ArtifactConfFormat)
	k := artifact.State(vmwcommon.ArtifactConfKeepRegistered)
	s := artifact.State(vmwcommon.ArtifactConfSkipExport)

	if f != "" && k != "true" && s == "false" {
		return nil, false, false, errors.New("To use this post-processor with exporting behavior you need set keep_registered as true")
	}

	// Make this part of config
	var attempts = 0
	var interval = 30 * time.Second
	var max_attempts = 6

	path := strings.Split(artifact.Id(), "::")
	datastore := path[0]
	folder := path[1]
	vmname := path[2]

	//log.Printf(spew.Sdump(path))
	//log.Printf("Inspecting %s", vmname)

	vc, err := govmomi.NewClient(
		context.Background(),
		p.url,
		p.config.Insecure,
	)
	if err != nil {
		return nil, false, false, fmt.Errorf("Error connecting to vsphere: %s", err)
	}

	defer vc.Logout(context.Background())

	finder := find.NewFinder(vc.Client, true)
	dc, err := finder.DefaultDatacenter(ctx)
	finder.SetDatacenter(dc)

	vm, err := finder.VirtualMachine(ctx, vmname)
	if err != nil {
		return nil, false, false, fmt.Errorf("Error finding vm: %s", err)
	}

	pc := property.DefaultCollector(vc.Client)

	var vmt mo.VirtualMachine


	err = pc.RetrieveOne(ctx, vm.Reference(), nil, &vmt)
	if err != nil {
		return nil, false, false, fmt.Errorf("Error getting vm detail: %s", err)
	}

	var success bool = false

	if vmt.Guest.ToolsStatus != "toolsOk" && vmt.Runtime.PowerState == "poweredOff" {
		fmt.Println("Guest tools not OK, and powered off.  " +
			   "Booting VM so that guest tools register " +
			   "with vCenter.")

		_,err = vm.PowerOn(ctx)
		if err != nil {
			log.Printf("Error Powering on VM: %s\n", err.Error())
		}

		for attempts <= max_attempts {
			err = pc.RetrieveOne(ctx, vm.Reference(), nil, &vmt)
			if err != nil {
				log.Printf("Error retrieving VM details: %s\n", err.Error())
			}
			//log.Printf("%s\n", vmt.Guest.ToolsStatus)
			if vmt.Guest.ToolsStatus == "toolsOk" {
				err = vm.ShutdownGuest(ctx)
				if err != nil {
					log.Printf("Error Powering on VM: %s\n", err.Error())
				}
				success = true
				break
			}
			attempts += 1
			time.Sleep(interval)
		}
	}

	//success	
	if success != true {
		log.Printf("Failed, returning error")
		return nil, false, false, fmt.Errorf("Unknown error registering guest tools\n")
	}

	//log.Printf(spew.Sdump(artifact))

	artifact = vsphere.NewArtifact(datastore, folder, vmname, artifact.Files())

	log.Printf("Success, returning artifact with id %s", artifact.BuilderId())
	return artifact, true, true, nil
}
