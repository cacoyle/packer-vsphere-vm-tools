# packer-vsphere-vm-tools
Packer post-processor to register vmware tools, allowing for guest customization of the output from the [vsphere](https://www.packer.io/docs/post-processors/vsphere.html) and [vsphere-template](https://www.packer.io/docs/post-processors/vsphere-template.html) post-processors during deployment.  Otherwise customspec fails because VMware thinks guest tools are not installed.

# Getting Started
These instructions will provide steps to build and use this post-processor in your vsphere packer builds.

## Prerequisites
* [Go](https://golang.org/pkg/) - 1.12.4 tested
* [Packer](https://www.packer.io) - 1.4.0 tested
* [VMware Player/Workstation](https://my.vmware.com/en/web/vmware/free#desktop_end_user_computing/vmware_workstation_player/14_0%7CPLAYER-1417%7Cproduct_downloads) - 14.1.7 tested
* [VMware VIX SDK](https://my.vmware.com/en/web/vmware/free#desktop_end_user_computing/vmware_workstation_player/14_0%7CPLAYER-1417%7Cdrivers_tools) - 1.15.0 tested
* [VMware vCenter and accompanying infra](https://www.vmware.com/products/vcenter-server.html) - ESXi 6.0-6.5 tested

## Building and installation
### Clone the project
`git clone https://github.com/cacoyle/packer-vsphere-vm-tools.git; cd packer-vsphere-vm-tools`

### Install dependencies
`go get -d ./...`

### Build the post-processor
`go build .`

### Install the post-processor
`cp packer-vsphere-vm-tools ~/.packer.d/plugins/packer-post-processor-vsphere-vm-tools`

## Using in packer build
```
{
  "variables": {
    "vcenter_url": "vcenter.local",
    "vcenter_username": "Administrator",
    "vcenter_password": "XXXXXXX"
  },
  "builders": [
    {
      "type": "vmware-iso",
      ....
    }
  ],
  "post-processors": [
    [
      {
        "type": "vsphere",
        ....
      },
      {
        "type": "vsphere-vm-tools",
        "host": "{{ user `vcenter_url` }}",
        "insecure": true,
        "username": "{{ user `vcenter_username` }}",
        "password": "{{ user `vcenter_password` }}",
        "keep_input_artifact": true
      },
      {
        "type": "vsphere-template",
        ....
      }
    ]
  ]
}
```

# Todo
* Make retry interval and max attempts part of configuration
* Fork/PR this feature to the packer/vsphere-template for review
