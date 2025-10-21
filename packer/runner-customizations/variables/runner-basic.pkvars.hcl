# GitHub Actions Runner - Basic Build
# Based on hv-packer Windows Server 2022 Standard

iso_url="https://software-static.download.prss.microsoft.com/sg/download/888969d5-f34g-4e03-ac9d-1f9786c66749/SERVER_EVAL_x64FRE_en-us.iso"
iso_checksum_type="sha256"
iso_checksum="3e4fa6d8507b554856fc9ca6079cc402df11a8b79344871669f0251535255325"

# Hyper-V Configuration
switch_name="Default Switch"
vlan_id=""

# VM Configuration
vm_name="github-runner-basic"
disk_size="40000"  # 40GB
output_directory="output-runner-basic"
memory="6144"  # 6GB
cpus="4"

# Use hv-packer's secondary ISO and sysprep configuration
secondary_iso_image="./hv-packer/extra/files/windows/2022/hyperv/std/secondary.iso"
sysprep_unattended="./hv-packer/extra/files/windows/2022/hyperv/std/unattend.xml"

# Upgrade timeout (minutes)
upgrade_timeout="240"
