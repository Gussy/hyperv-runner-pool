# Shared variables for GitHub Actions Runner builds
# These are compatible with hv-packer's variable structure

variable "memory" {
  type        = string
  description = "VM memory in MB"
}

variable "cpus" {
  type        = string
  description = "Number of CPU cores"
}

variable "disk_size" {
  type        = string
  description = "Disk size in MB"
}

variable "iso_checksum" {
  type        = string
  description = "SHA256 checksum of the ISO"
}

variable "iso_checksum_type" {
  type        = string
  description = "Checksum type (sha256)"
}

variable "iso_url" {
  type        = string
  description = "Windows Server ISO URL"
}

variable "output_directory" {
  type        = string
  description = "Output directory for built VM"
}

variable "secondary_iso_image" {
  type        = string
  description = "Path to secondary ISO (autounattend)"
}

variable "switch_name" {
  type        = string
  description = "Hyper-V switch name"
}

variable "sysprep_unattended" {
  type        = string
  description = "Path to sysprep unattend.xml"
}

variable "upgrade_timeout" {
  type        = string
  default     = "240"
  description = "Windows Update timeout in minutes"
}

variable "vlan_id" {
  type        = string
  default     = ""
  description = "VLAN ID (optional)"
}

variable "vm_name" {
  type        = string
  description = "Virtual machine name"
}
