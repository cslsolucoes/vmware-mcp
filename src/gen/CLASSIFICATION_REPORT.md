# Fase 0 — relatório de classificação (gerado, para revisão humana)

527 métodos candidatos encontrados. **Nada foi gerado em tools/ a partir disto ainda** — ver o plano "MCPVMWare2026-08-10-175300-plano-cobertura-completa-api-codegen.plan.md" §Fase 0.

## object (309 métodos)

| Receiver | Método | Tier | Modo | Tool proposta | Regra |
|---|---|---|---|---|---|
| AuthorizationManager | AddRole(name string, ids []string) | tier2 | vsphere-general | `vmware_authorization_add_role` | fail-safe default (no pattern matched) |
| AuthorizationManager | DisableMethods(entity []types.ManagedObjectReference, method []DisabledMethodRequest, source string) | tier2 | vsphere-general | `vmware_authorization_disable_methods` | name matches tier2 pattern |
| AuthorizationManager | EnableMethods(entity []types.ManagedObjectReference, method []string, source string) | tier2 | vsphere-general | `vmware_authorization_enable_methods` | fail-safe default (no pattern matched) |
| AuthorizationManager | FetchUserPrivilegeOnEntities(entities []types.ManagedObjectReference, userName string) | — | vsphere-general | `vmware_authorization_fetch_user_privilege_on_entities` | name matches read-only pattern |
| AuthorizationManager | HasPrivilegeOnEntity(entity types.ManagedObjectReference, sessionID string, privID []string) | — | vsphere-general | `vmware_authorization_has_privilege_on_entity` | name matches read-only pattern |
| AuthorizationManager | HasUserPrivilegeOnEntities(entities []types.ManagedObjectReference, userName string, privID []string) | — | vsphere-general | `vmware_authorization_has_user_privilege_on_entities` | name matches read-only pattern |
| AuthorizationManager | RemoveEntityPermission(entity types.ManagedObjectReference, user string, isGroup bool) | tier1 | vsphere-general | `vmware_authorization_remove_entity_permission` | name matches tier1 pattern |
| AuthorizationManager | RemoveRole(id int32, failIfUsed bool) | tier1 | vsphere-general | `vmware_authorization_remove_role` | name matches tier1 pattern |
| AuthorizationManager | RetrieveAllPermissions() | — | vsphere-general | `vmware_authorization_retrieve_all_permissions` | name matches read-only pattern |
| AuthorizationManager | RetrieveEntityPermissions(entity types.ManagedObjectReference, inherited bool) | — | vsphere-general | `vmware_authorization_retrieve_entity_permissions` | name matches read-only pattern |
| AuthorizationManager | RetrieveRolePermissions(id int32) | — | vsphere-general | `vmware_authorization_retrieve_role_permissions` | name matches read-only pattern |
| AuthorizationManager | RoleList() | tier2 | vsphere-general | `vmware_authorization_role_list` | fail-safe default (no pattern matched) |
| AuthorizationManager | SetEntityPermissions(entity types.ManagedObjectReference, permission []types.Permission) | tier2 | vsphere-general | `vmware_authorization_set_entity_permissions` | fail-safe default (no pattern matched) |
| AuthorizationManager | UpdateRole(id int32, name string, ids []string) | tier2 | vsphere-general | `vmware_authorization_update_role` | fail-safe default (no pattern matched) |
| ClusterComputeResource | AddHost(spec types.HostConnectSpec, asConnected bool, license *string, resourcePool *types.ManagedObjectReference) | tier2 | vcenter-only | `vmware_cluster_add_host` | fail-safe default (no pattern matched) |
| ClusterComputeResource | Configuration() | tier2 | vcenter-only | `vmware_cluster_configuration` | fail-safe default (no pattern matched) |
| ClusterComputeResource | MoveInto(hosts ...*HostSystem) | tier2 | vcenter-only | `vmware_cluster_move_into` | fail-safe default (no pattern matched) |
| ClusterComputeResource | PlaceVm(spec types.PlacementSpec) | tier2 | vcenter-only | `vmware_cluster_place_vm` | fail-safe default (no pattern matched) |
| Common | Destroy() | tier1 | vsphere-general | `vmware_common_destroy` | name matches tier1 pattern |
| Common | ObjectName() | tier2 | vsphere-general | `vmware_common_object_name` | fail-safe default (no pattern matched) |
| Common | Properties(r types.ManagedObjectReference, ps []string, dst any) | tier2 | vsphere-general | `vmware_common_properties` | fail-safe default (no pattern matched) |
| Common | Rename(name string) | tier2 | vsphere-general | `vmware_common_rename` | fail-safe default (no pattern matched) |
| Common | SetCustomValue(key string, value string) | tier2 | vsphere-general | `vmware_common_set_custom_value` | fail-safe default (no pattern matched) |
| ComputeResource | Datastores() | tier2 | vsphere-general | `vmware_compute_resource_datastores` | fail-safe default (no pattern matched) |
| ComputeResource | EnvironmentBrowser() | tier2 | vsphere-general | `vmware_compute_resource_environment_browser` | fail-safe default (no pattern matched) |
| ComputeResource | Hosts() | tier2 | vsphere-general | `vmware_compute_resource_hosts` | fail-safe default (no pattern matched) |
| ComputeResource | Reconfigure(spec types.BaseComputeResourceConfigSpec, modify bool) | tier2 | vsphere-general | `vmware_compute_resource_reconfigure` | fail-safe default (no pattern matched) |
| ComputeResource | ResourcePool() | tier2 | vsphere-general | `vmware_compute_resource_resource_pool` | fail-safe default (no pattern matched) |
| CustomFieldsManager | Add(name string, moType string, fieldDefPolicy *types.PrivilegePolicyDef, fieldPolicy *types.PrivilegePolicyDef) | tier2 | vcenter-only | `vmware_custom_field_add` | fail-safe default (no pattern matched) |
| CustomFieldsManager | Field() | tier2 | vcenter-only | `vmware_custom_field_field` | fail-safe default (no pattern matched) |
| CustomFieldsManager | FindKey(name string) | — | vcenter-only | `vmware_custom_field_find_key` | name matches read-only pattern |
| CustomFieldsManager | Remove(key int32) | tier1 | vcenter-only | `vmware_custom_field_remove` | name matches tier1 pattern |
| CustomFieldsManager | Rename(key int32, name string) | tier2 | vcenter-only | `vmware_custom_field_rename` | fail-safe default (no pattern matched) |
| CustomFieldsManager | Set(entity types.ManagedObjectReference, key int32, value string) | tier2 | vcenter-only | `vmware_custom_field_set` | fail-safe default (no pattern matched) |
| CustomizationSpecManager | CreateCustomizationSpec(item types.CustomizationSpecItem) | tier2 | vcenter-only | `vmware_customization_spec_create_customization_spec` | fail-safe default (no pattern matched) |
| CustomizationSpecManager | CustomizationSpecItemToXml(item types.CustomizationSpecItem) | tier2 | vcenter-only | `vmware_customization_spec_customization_spec_item_to_xml` | fail-safe default (no pattern matched) |
| CustomizationSpecManager | DeleteCustomizationSpec(name string) | tier1 | vcenter-only | `vmware_customization_spec_delete_customization_spec` | name matches tier1 pattern |
| CustomizationSpecManager | DoesCustomizationSpecExist(name string) | tier2 | vcenter-only | `vmware_customization_spec_does_customization_spec_exist` | fail-safe default (no pattern matched) |
| CustomizationSpecManager | DuplicateCustomizationSpec(name string, newName string) | tier2 | vcenter-only | `vmware_customization_spec_duplicate_customization_spec` | fail-safe default (no pattern matched) |
| CustomizationSpecManager | GetCustomizationSpec(name string) | — | vcenter-only | `vmware_customization_spec_get_customization_spec` | name matches read-only pattern |
| CustomizationSpecManager | Info() | tier2 | vcenter-only | `vmware_customization_spec_info` | fail-safe default (no pattern matched) |
| CustomizationSpecManager | OverwriteCustomizationSpec(item types.CustomizationSpecItem) | tier2 | vcenter-only | `vmware_customization_spec_overwrite_customization_spec` | fail-safe default (no pattern matched) |
| CustomizationSpecManager | RenameCustomizationSpec(name string, newName string) | tier2 | vcenter-only | `vmware_customization_spec_rename_customization_spec` | fail-safe default (no pattern matched) |
| CustomizationSpecManager | XmlToCustomizationSpecItem(xml string) | tier2 | vcenter-only | `vmware_customization_spec_xml_to_customization_spec_item` | fail-safe default (no pattern matched) |
| Datacenter | Destroy() | tier1 | vsphere-general | `vmware_datacenter_destroy` | name matches tier1 pattern |
| Datacenter | Folders() | tier2 | vsphere-general | `vmware_datacenter_folders` | fail-safe default (no pattern matched) |
| Datacenter | PowerOnVM(vm []types.ManagedObjectReference, option ...types.BaseOptionValue) | tier2 | vsphere-general | `vmware_datacenter_power_on_vm` | fail-safe default (no pattern matched) |
| Datastore | AttachedClusterHosts(cluster *ComputeResource) | tier2 | vsphere-general | `vmware_datastore_attached_cluster_hosts` | fail-safe default (no pattern matched) |
| Datastore | AttachedHosts() | tier2 | vsphere-general | `vmware_datastore_attached_hosts` | fail-safe default (no pattern matched) |
| Datastore | Browser() | tier2 | vsphere-general | `vmware_datastore_browser` | fail-safe default (no pattern matched) |
| Datastore | Download(path string, param *soap.Download) | tier2 | vsphere-general | `vmware_datastore_download` | fail-safe default (no pattern matched) |
| Datastore | DownloadFile(path string, file string, param *soap.Download) | tier2 | vsphere-general | `vmware_datastore_download_file` | fail-safe default (no pattern matched) |
| Datastore | FindInventoryPath() | — | vsphere-general | `vmware_datastore_find_inventory_path` | name matches read-only pattern |
| Datastore | HostContext(host *HostSystem) | tier2 | vsphere-general | `vmware_datastore_host_context` | fail-safe default (no pattern matched) |
| Datastore | Open(name string) | tier2 | vsphere-general | `vmware_datastore_open` | fail-safe default (no pattern matched) |
| Datastore | ServiceTicket(path string, method string) | tier2 | vsphere-general | `vmware_datastore_service_ticket` | fail-safe default (no pattern matched) |
| Datastore | Stat(file string) | tier2 | vsphere-general | `vmware_datastore_stat` | fail-safe default (no pattern matched) |
| Datastore | Type() | tier2 | vsphere-general | `vmware_datastore_type` | fail-safe default (no pattern matched) |
| Datastore | Upload(f io.Reader, path string, param *soap.Upload) | tier2 | vsphere-general | `vmware_datastore_upload` | fail-safe default (no pattern matched) |
| DatastoreFileManager | Copy(src string, dst string) | tier2 | vsphere-general | `vmware_datastore_file_copy` | fail-safe default (no pattern matched) |
| DatastoreFileManager | CopyFile(src string, dst string) | tier2 | vsphere-general | `vmware_datastore_file_copy_file` | fail-safe default (no pattern matched) |
| DatastoreFileManager | Delete(name string) | tier1 | vsphere-general | `vmware_datastore_file_delete` | name matches tier1 pattern |
| DatastoreFileManager | DeleteFile(name string) | tier1 | vsphere-general | `vmware_datastore_file_delete_file` | name matches tier1 pattern |
| DatastoreFileManager | DeleteVirtualDisk(name string) | tier1 | vsphere-general | `vmware_datastore_file_delete_virtual_disk` | name matches tier1 pattern |
| DatastoreFileManager | Move(src string, dst string) | tier2 | vsphere-general | `vmware_datastore_file_move` | fail-safe default (no pattern matched) |
| DatastoreFileManager | MoveFile(src string, dst string) | tier2 | vsphere-general | `vmware_datastore_file_move_file` | fail-safe default (no pattern matched) |
| DatastoreFileManager | WithProgress(s progress.Sinker) | tier2 | vsphere-general | `vmware_datastore_file_with_progress` | fail-safe default (no pattern matched) |
| DatastoreNamespaceManager | ConvertNamespacePathToUuidPath(dc *Datacenter, datastoreURL string) | tier2 | vsphere-general | `vmware_datastore_namespace_manager_convert_namespace_path_to_uuid_path` | fail-safe default (no pattern matched) |
| DatastoreNamespaceManager | CreateDirectory(ds *Datastore, displayName string, policy string) | tier2 | vsphere-general | `vmware_datastore_namespace_manager_create_directory` | fail-safe default (no pattern matched) |
| DatastoreNamespaceManager | DeleteDirectory(dc *Datacenter, datastorePath string) | tier1 | vsphere-general | `vmware_datastore_namespace_manager_delete_directory` | name matches tier1 pattern |
| DiagnosticLog | Copy(w io.Writer) | tier2 | vsphere-general | `vmware_diagnostic_log_copy` | fail-safe default (no pattern matched) |
| DiagnosticLog | Seek(nlines int32) | tier2 | vsphere-general | `vmware_diagnostic_log_seek` | fail-safe default (no pattern matched) |
| DiagnosticManager | BrowseLog(host *HostSystem, key string, start int32, lines int32) | tier2 | vsphere-general | `vmware_diagnostic_browse_log` | fail-safe default (no pattern matched) |
| DiagnosticManager | GenerateLogBundles(includeDefault bool, host []*HostSystem) | tier2 | vsphere-general | `vmware_diagnostic_generate_log_bundles` | fail-safe default (no pattern matched) |
| DiagnosticManager | Log(host *HostSystem, key string) | tier2 | vsphere-general | `vmware_diagnostic_log` | fail-safe default (no pattern matched) |
| DiagnosticManager | QueryDescriptions(host *HostSystem) | — | vsphere-general | `vmware_diagnostic_query_descriptions` | name matches read-only pattern |
| DistributedVirtualPortgroup | EthernetCardBackingInfo() | tier2 | vcenter-only | `vmware_dvpg_ethernet_card_backing_info` | fail-safe default (no pattern matched) |
| DistributedVirtualPortgroup | Reconfigure(spec types.DVPortgroupConfigSpec) | tier2 | vcenter-only | `vmware_dvpg_reconfigure` | fail-safe default (no pattern matched) |
| DistributedVirtualSwitch | AddPortgroup(spec []types.DVPortgroupConfigSpec) | tier2 | vcenter-only | `vmware_dvs_add_portgroup` | fail-safe default (no pattern matched) |
| DistributedVirtualSwitch | EthernetCardBackingInfo() | tier2 | vcenter-only | `vmware_dvs_ethernet_card_backing_info` | fail-safe default (no pattern matched) |
| DistributedVirtualSwitch | FetchDVPorts(criteria *types.DistributedVirtualSwitchPortCriteria) | — | vcenter-only | `vmware_dvs_fetch_dvports` | name matches read-only pattern |
| DistributedVirtualSwitch | Reconfigure(spec types.BaseDVSConfigSpec) | tier2 | vcenter-only | `vmware_dvs_reconfigure` | fail-safe default (no pattern matched) |
| DistributedVirtualSwitch | ReconfigureDVPort(spec []types.DVPortConfigSpec) | tier2 | vcenter-only | `vmware_dvs_reconfigure_dvport` | fail-safe default (no pattern matched) |
| DistributedVirtualSwitch | ReconfigureLACP(spec []types.VMwareDvsLacpGroupSpec) | tier2 | vcenter-only | `vmware_dvs_reconfigure_lacp` | fail-safe default (no pattern matched) |
| EnvironmentBrowser | QueryConfigOption(spec *types.EnvironmentBrowserConfigOptionQuerySpec) | — | vsphere-general | `vmware_environment_query_config_option` | name matches read-only pattern |
| EnvironmentBrowser | QueryConfigOptionDescriptor() | — | vsphere-general | `vmware_environment_query_config_option_descriptor` | name matches read-only pattern |
| EnvironmentBrowser | QueryConfigTarget(host *HostSystem) | — | vsphere-general | `vmware_environment_query_config_target` | name matches read-only pattern |
| EnvironmentBrowser | QueryTargetCapabilities(host *HostSystem) | — | vsphere-general | `vmware_environment_query_target_capabilities` | name matches read-only pattern |
| ExtensionManager | Find(key string) | — | vcenter-only | `vmware_extension_find` | name matches read-only pattern |
| ExtensionManager | List() | — | vcenter-only | `vmware_extension_list` | name matches read-only pattern |
| ExtensionManager | Register(extension types.Extension) | tier2 | vcenter-only | `vmware_extension_register` | fail-safe default (no pattern matched) |
| ExtensionManager | SetCertificate(key string, certificatePem string) | tier2 | vcenter-only | `vmware_extension_set_certificate` | fail-safe default (no pattern matched) |
| ExtensionManager | Unregister(key string) | tier1 | vcenter-only | `vmware_extension_unregister` | name matches tier1 pattern |
| ExtensionManager | Update(extension types.Extension) | tier2 | vcenter-only | `vmware_extension_update` | fail-safe default (no pattern matched) |
| FileManager | CopyDatastoreFile(sourceName string, sourceDatacenter *Datacenter, destinationName string, destinationDatacenter *Datacenter, force bool) | tier2 | vsphere-general | `vmware_file_copy_datastore_file` | fail-safe default (no pattern matched) |
| FileManager | DeleteDatastoreFile(name string, dc *Datacenter) | tier1 | vsphere-general | `vmware_file_delete_datastore_file` | name matches tier1 pattern |
| FileManager | MakeDirectory(name string, dc *Datacenter, createParentDirectories bool) | tier2 | vsphere-general | `vmware_file_make_directory` | fail-safe default (no pattern matched) |
| FileManager | MoveDatastoreFile(sourceName string, sourceDatacenter *Datacenter, destinationName string, destinationDatacenter *Datacenter, force bool) | tier2 | vsphere-general | `vmware_file_move_datastore_file` | fail-safe default (no pattern matched) |
| Folder | AddStandaloneHost(spec types.HostConnectSpec, addConnected bool, license *string, compResSpec *types.BaseComputeResourceConfigSpec) | tier2 | vsphere-general | `vmware_folder_add_standalone_host` | fail-safe default (no pattern matched) |
| Folder | Children() | tier2 | vsphere-general | `vmware_folder_children` | fail-safe default (no pattern matched) |
| Folder | CreateCluster(cluster string, spec types.ClusterConfigSpecEx) | tier2 | vsphere-general | `vmware_folder_create_cluster` | fail-safe default (no pattern matched) |
| Folder | CreateDVS(spec types.DVSCreateSpec) | tier2 | vsphere-general | `vmware_folder_create_dvs` | fail-safe default (no pattern matched) |
| Folder | CreateDatacenter(datacenter string) | tier2 | vsphere-general | `vmware_folder_create_datacenter` | fail-safe default (no pattern matched) |
| Folder | CreateFolder(name string) | tier2 | vsphere-general | `vmware_folder_create_folder` | fail-safe default (no pattern matched) |
| Folder | CreateStoragePod(name string) | tier2 | vsphere-general | `vmware_folder_create_storage_pod` | fail-safe default (no pattern matched) |
| Folder | CreateVM(config types.VirtualMachineConfigSpec, pool *ResourcePool, host *HostSystem) | tier2 | vsphere-general | `vmware_folder_create_vm` | fail-safe default (no pattern matched) |
| Folder | MoveInto(list []types.ManagedObjectReference) | tier2 | vsphere-general | `vmware_folder_move_into` | fail-safe default (no pattern matched) |
| Folder | PlaceVmsXCluster(spec types.PlaceVmsXClusterSpec) | tier2 | vsphere-general | `vmware_folder_place_vms_xcluster` | fail-safe default (no pattern matched) |
| Folder | RegisterVM(path string, name string, asTemplate bool, pool *ResourcePool, host *HostSystem) | tier2 | vsphere-general | `vmware_folder_register_vm` | fail-safe default (no pattern matched) |
| HostAccountManager | Create(user *types.HostAccountSpec) | tier2 | vsphere-general | `vmware_host_account_create` | fail-safe default (no pattern matched) |
| HostAccountManager | Remove(userName string) | tier1 | vsphere-general | `vmware_host_account_remove` | name matches tier1 pattern |
| HostAccountManager | Update(user *types.HostAccountSpec) | tier2 | vsphere-general | `vmware_host_account_update` | fail-safe default (no pattern matched) |
| HostCertificateManager | CertificateInfo() | tier2 | vsphere-general | `vmware_host_certificate_certificate_info` | fail-safe default (no pattern matched) |
| HostCertificateManager | GenerateCertificateSigningRequest(useIPAddressAsCommonName bool) | tier2 | vsphere-general | `vmware_host_certificate_generate_certificate_signing_request` | fail-safe default (no pattern matched) |
| HostCertificateManager | GenerateCertificateSigningRequestByDn(distinguishedName string) | tier2 | vsphere-general | `vmware_host_certificate_generate_certificate_signing_request_by_dn` | fail-safe default (no pattern matched) |
| HostCertificateManager | InstallServerCertificate(cert string) | tier2 | vsphere-general | `vmware_host_certificate_install_server_certificate` | fail-safe default (no pattern matched) |
| HostCertificateManager | ListCACertificateRevocationLists() | — | vsphere-general | `vmware_host_certificate_list_cacertificate_revocation_lists` | name matches read-only pattern |
| HostCertificateManager | ListCACertificates() | — | vsphere-general | `vmware_host_certificate_list_cacertificates` | name matches read-only pattern |
| HostCertificateManager | ReplaceCACertificatesAndCRLs(caCert []string, caCrl []string) | tier2 | vsphere-general | `vmware_host_certificate_replace_cacertificates_and_crls` | fail-safe default (no pattern matched) |
| HostConfigManager | AccountManager() | tier2 | vsphere-general | `vmware_host_config_account_manager` | fail-safe default (no pattern matched) |
| HostConfigManager | CertificateManager() | tier2 | vsphere-general | `vmware_host_config_certificate_manager` | fail-safe default (no pattern matched) |
| HostConfigManager | DatastoreSystem() | tier2 | vsphere-general | `vmware_host_config_datastore_system` | fail-safe default (no pattern matched) |
| HostConfigManager | DateTimeSystem() | tier2 | vsphere-general | `vmware_host_config_date_time_system` | fail-safe default (no pattern matched) |
| HostConfigManager | FirewallSystem() | tier2 | vsphere-general | `vmware_host_config_firewall_system` | fail-safe default (no pattern matched) |
| HostConfigManager | NetworkSystem() | tier2 | vsphere-general | `vmware_host_config_network_system` | fail-safe default (no pattern matched) |
| HostConfigManager | OptionManager() | tier2 | vsphere-general | `vmware_host_config_option_manager` | fail-safe default (no pattern matched) |
| HostConfigManager | ServiceSystem() | tier2 | vsphere-general | `vmware_host_config_service_system` | fail-safe default (no pattern matched) |
| HostConfigManager | StorageSystem() | tier2 | vsphere-general | `vmware_host_config_storage_system` | fail-safe default (no pattern matched) |
| HostConfigManager | VirtualNicManager() | tier2 | vsphere-general | `vmware_host_config_virtual_nic_manager` | fail-safe default (no pattern matched) |
| HostConfigManager | VsanInternalSystem() | tier2 | vsphere-general | `vmware_host_config_vsan_internal_system` | fail-safe default (no pattern matched) |
| HostConfigManager | VsanSystem() | tier2 | vsphere-general | `vmware_host_config_vsan_system` | fail-safe default (no pattern matched) |
| HostDatastoreBrowser | SearchDatastore(datastorePath string, searchSpec *types.HostDatastoreBrowserSearchSpec) | — | vsphere-general | `vmware_datastore_browser_search_datastore` | name matches read-only pattern |
| HostDatastoreBrowser | SearchDatastoreSubFolders(datastorePath string, searchSpec *types.HostDatastoreBrowserSearchSpec) | — | vsphere-general | `vmware_datastore_browser_search_datastore_sub_folders` | name matches read-only pattern |
| HostDatastoreSystem | CreateLocalDatastore(name string, path string) | tier2 | vsphere-general | `vmware_host_datastore_create_local_datastore` | fail-safe default (no pattern matched) |
| HostDatastoreSystem | CreateNasDatastore(spec types.HostNasVolumeSpec) | tier2 | vsphere-general | `vmware_host_datastore_create_nas_datastore` | fail-safe default (no pattern matched) |
| HostDatastoreSystem | CreateVmfsDatastore(spec types.VmfsDatastoreCreateSpec) | tier2 | vsphere-general | `vmware_host_datastore_create_vmfs_datastore` | fail-safe default (no pattern matched) |
| HostDatastoreSystem | QueryAvailableDisksForVmfs() | — | vsphere-general | `vmware_host_datastore_query_available_disks_for_vmfs` | name matches read-only pattern |
| HostDatastoreSystem | QueryVmfsDatastoreCreateOptions(devicePath string) | — | vsphere-general | `vmware_host_datastore_query_vmfs_datastore_create_options` | name matches read-only pattern |
| HostDatastoreSystem | Remove(ds *Datastore) | tier1 | vsphere-general | `vmware_host_datastore_remove` | name matches tier1 pattern |
| HostDatastoreSystem | ResignatureUnresolvedVmfsVolumes(devicePaths []string) | tier2 | vsphere-general | `vmware_host_datastore_resignature_unresolved_vmfs_volumes` | fail-safe default (no pattern matched) |
| HostDateTimeSystem | Query() | — | vsphere-general | `vmware_host_datetime_query` | name matches read-only pattern |
| HostDateTimeSystem | Update(date time.Time) | tier2 | vsphere-general | `vmware_host_datetime_update` | fail-safe default (no pattern matched) |
| HostDateTimeSystem | UpdateConfig(config types.HostDateTimeConfig) | tier2 | vsphere-general | `vmware_host_datetime_update_config` | fail-safe default (no pattern matched) |
| HostFirewallSystem | DisableRuleset(id string) | tier2 | vsphere-general | `vmware_host_firewall_disable_ruleset` | name matches tier2 pattern |
| HostFirewallSystem | EnableRuleset(id string) | tier2 | vsphere-general | `vmware_host_firewall_enable_ruleset` | fail-safe default (no pattern matched) |
| HostFirewallSystem | Info() | tier2 | vsphere-general | `vmware_host_firewall_info` | fail-safe default (no pattern matched) |
| HostFirewallSystem | Refresh() | tier2 | vsphere-general | `vmware_host_firewall_refresh` | fail-safe default (no pattern matched) |
| HostNetworkSystem | AddPortGroup(portgrp types.HostPortGroupSpec) | tier2 | vsphere-general | `vmware_host_network_add_port_group` | fail-safe default (no pattern matched) |
| HostNetworkSystem | AddServiceConsoleVirtualNic(portgroup string, nic types.HostVirtualNicSpec) | tier2 | vsphere-general | `vmware_host_network_add_service_console_virtual_nic` | fail-safe default (no pattern matched) |
| HostNetworkSystem | AddVirtualNic(portgroup string, nic types.HostVirtualNicSpec) | tier2 | vsphere-general | `vmware_host_network_add_virtual_nic` | fail-safe default (no pattern matched) |
| HostNetworkSystem | AddVirtualSwitch(vswitchName string, spec *types.HostVirtualSwitchSpec) | tier2 | vsphere-general | `vmware_host_network_add_virtual_switch` | fail-safe default (no pattern matched) |
| HostNetworkSystem | QueryNetworkHint(device []string) | — | vsphere-general | `vmware_host_network_query_network_hint` | name matches read-only pattern |
| HostNetworkSystem | RefreshNetworkSystem() | tier2 | vsphere-general | `vmware_host_network_refresh_network_system` | fail-safe default (no pattern matched) |
| HostNetworkSystem | RemovePortGroup(pgName string) | tier1 | vsphere-general | `vmware_host_network_remove_port_group` | name matches tier1 pattern |
| HostNetworkSystem | RemoveServiceConsoleVirtualNic(device string) | tier1 | vsphere-general | `vmware_host_network_remove_service_console_virtual_nic` | name matches tier1 pattern |
| HostNetworkSystem | RemoveVirtualNic(device string) | tier1 | vsphere-general | `vmware_host_network_remove_virtual_nic` | name matches tier1 pattern |
| HostNetworkSystem | RemoveVirtualSwitch(vswitchName string) | tier1 | vsphere-general | `vmware_host_network_remove_virtual_switch` | name matches tier1 pattern |
| HostNetworkSystem | RestartServiceConsoleVirtualNic(device string) | tier2 | vsphere-general | `vmware_host_network_restart_service_console_virtual_nic` | name matches tier2 pattern |
| HostNetworkSystem | UpdateConsoleIpRouteConfig(config types.BaseHostIpRouteConfig) | tier2 | vsphere-general | `vmware_host_network_update_console_ip_route_config` | fail-safe default (no pattern matched) |
| HostNetworkSystem | UpdateDnsConfig(config types.BaseHostDnsConfig) | tier2 | vsphere-general | `vmware_host_network_update_dns_config` | fail-safe default (no pattern matched) |
| HostNetworkSystem | UpdateIpRouteConfig(config types.BaseHostIpRouteConfig) | tier2 | vsphere-general | `vmware_host_network_update_ip_route_config` | fail-safe default (no pattern matched) |
| HostNetworkSystem | UpdateIpRouteTableConfig(config types.HostIpRouteTableConfig) | tier2 | vsphere-general | `vmware_host_network_update_ip_route_table_config` | fail-safe default (no pattern matched) |
| HostNetworkSystem | UpdateNetworkConfig(config types.HostNetworkConfig, changeMode string) | tier2 | vsphere-general | `vmware_host_network_update_network_config` | fail-safe default (no pattern matched) |
| HostNetworkSystem | UpdatePhysicalNicLinkSpeed(device string, linkSpeed *types.PhysicalNicLinkInfo) | tier2 | vsphere-general | `vmware_host_network_update_physical_nic_link_speed` | fail-safe default (no pattern matched) |
| HostNetworkSystem | UpdatePortGroup(pgName string, portgrp types.HostPortGroupSpec) | tier2 | vsphere-general | `vmware_host_network_update_port_group` | fail-safe default (no pattern matched) |
| HostNetworkSystem | UpdateServiceConsoleVirtualNic(device string, nic types.HostVirtualNicSpec) | tier2 | vsphere-general | `vmware_host_network_update_service_console_virtual_nic` | fail-safe default (no pattern matched) |
| HostNetworkSystem | UpdateVirtualNic(device string, nic types.HostVirtualNicSpec) | tier2 | vsphere-general | `vmware_host_network_update_virtual_nic` | fail-safe default (no pattern matched) |
| HostNetworkSystem | UpdateVirtualSwitch(vswitchName string, spec types.HostVirtualSwitchSpec) | tier2 | vsphere-general | `vmware_host_network_update_virtual_switch` | fail-safe default (no pattern matched) |
| HostServiceSystem | Restart(id string) | tier2 | vsphere-general | `vmware_host_service_restart` | name matches tier2 pattern |
| HostServiceSystem | Service() | tier2 | vsphere-general | `vmware_host_service_service` | fail-safe default (no pattern matched) |
| HostServiceSystem | Start(id string) | tier2 | vsphere-general | `vmware_host_service_start` | fail-safe default (no pattern matched) |
| HostServiceSystem | Stop(id string) | tier2 | vsphere-general | `vmware_host_service_stop` | name matches tier2 pattern |
| HostServiceSystem | UpdatePolicy(id string, policy string) | tier2 | vsphere-general | `vmware_host_service_update_policy` | fail-safe default (no pattern matched) |
| HostStorageSystem | AttachScsiLun(uuid string) | tier2 | vsphere-general | `vmware_host_storage_attach_scsi_lun` | fail-safe default (no pattern matched) |
| HostStorageSystem | ComputeDiskPartitionInfo(devicePath string, layout types.HostDiskPartitionLayout) | tier2 | vsphere-general | `vmware_host_storage_compute_disk_partition_info` | fail-safe default (no pattern matched) |
| HostStorageSystem | MarkAsLocal(uuid string) | tier2 | vsphere-general | `vmware_host_storage_mark_as_local` | fail-safe default (no pattern matched) |
| HostStorageSystem | MarkAsNonLocal(uuid string) | tier2 | vsphere-general | `vmware_host_storage_mark_as_non_local` | fail-safe default (no pattern matched) |
| HostStorageSystem | MarkAsNonSsd(uuid string) | tier2 | vsphere-general | `vmware_host_storage_mark_as_non_ssd` | fail-safe default (no pattern matched) |
| HostStorageSystem | MarkAsSsd(uuid string) | tier2 | vsphere-general | `vmware_host_storage_mark_as_ssd` | fail-safe default (no pattern matched) |
| HostStorageSystem | QueryUnresolvedVmfsVolumes() | — | vsphere-general | `vmware_host_storage_query_unresolved_vmfs_volumes` | name matches read-only pattern |
| HostStorageSystem | Refresh() | tier2 | vsphere-general | `vmware_host_storage_refresh` | fail-safe default (no pattern matched) |
| HostStorageSystem | RescanAllHba() | tier2 | vsphere-general | `vmware_host_storage_rescan_all_hba` | fail-safe default (no pattern matched) |
| HostStorageSystem | RescanVmfs() | tier2 | vsphere-general | `vmware_host_storage_rescan_vmfs` | fail-safe default (no pattern matched) |
| HostStorageSystem | RetrieveDiskPartitionInfo(devicePath string) | — | vsphere-general | `vmware_host_storage_retrieve_disk_partition_info` | name matches read-only pattern |
| HostStorageSystem | UnmountVmfsVolume(vmfsUuid string) | tier1 | vsphere-general | `vmware_host_storage_unmount_vmfs_volume` | name matches tier1 pattern |
| HostStorageSystem | UpdateDiskPartitionInfo(devicePath string, spec types.HostDiskPartitionSpec) | tier2 | vsphere-general | `vmware_host_storage_update_disk_partition_info` | fail-safe default (no pattern matched) |
| HostSystem | Disconnect() | tier1 | vsphere-general | `vmware_host_disconnect` | name matches tier1 pattern |
| HostSystem | EnterMaintenanceMode(timeout int32, evacuate bool, spec *types.HostMaintenanceSpec) | tier2 | vsphere-general | `vmware_host_enter_maintenance_mode` | fail-safe default (no pattern matched) |
| HostSystem | ExitMaintenanceMode(timeout int32) | tier2 | vsphere-general | `vmware_host_exit_maintenance_mode` | fail-safe default (no pattern matched) |
| HostSystem | ResourcePool() | tier2 | vsphere-general | `vmware_host_resource_pool` | fail-safe default (no pattern matched) |
| HostSystem | UpdatePodVMProperty(propertyPath string, podVMInfo types.HostRuntimeInfoPodVMInfo) | tier2 | vsphere-general | `vmware_host_update_pod_vmproperty` | fail-safe default (no pattern matched) |
| HostVirtualNicManager | DeselectVnic(nicType string, device string) | tier2 | vsphere-general | `vmware_host_nic_deselect_vnic` | fail-safe default (no pattern matched) |
| HostVirtualNicManager | Info() | tier2 | vsphere-general | `vmware_host_nic_info` | fail-safe default (no pattern matched) |
| HostVirtualNicManager | SelectVnic(nicType string, device string) | tier2 | vsphere-general | `vmware_host_nic_select_vnic` | fail-safe default (no pattern matched) |
| HostVsanInternalSystem | DeleteVsanObjects(uuids []string, force *bool) | tier1 | vsphere-general | `vmware_host_vsan_internal_delete_vsan_objects` | name matches tier1 pattern |
| HostVsanInternalSystem | GetVsanObjExtAttrs(uuids []string) | — | vsphere-general | `vmware_host_vsan_internal_get_vsan_obj_ext_attrs` | name matches read-only pattern |
| HostVsanInternalSystem | QueryVsanObjectUuidsByFilter(uuids []string, limit int32, version int32) | — | vsphere-general | `vmware_host_vsan_internal_query_vsan_object_uuids_by_filter` | name matches read-only pattern |
| HostVsanSystem | Update(config types.VsanHostConfigInfo) | tier2 | vsphere-general | `vmware_host_vsan_update` | fail-safe default (no pattern matched) |
| Network | EthernetCardBackingInfo() | tier2 | vsphere-general | `vmware_network_ethernet_card_backing_info` | fail-safe default (no pattern matched) |
| OpaqueNetwork | EthernetCardBackingInfo() | tier2 | vsphere-general | `vmware_opaque_network_ethernet_card_backing_info` | fail-safe default (no pattern matched) |
| OpaqueNetwork | Summary() | tier2 | vsphere-general | `vmware_opaque_network_summary` | fail-safe default (no pattern matched) |
| OptionManager | Query(name string) | — | vsphere-general | `vmware_option_query` | name matches read-only pattern |
| OptionManager | Update(value []types.BaseOptionValue) | tier2 | vsphere-general | `vmware_option_update` | fail-safe default (no pattern matched) |
| ResourcePool | Create(name string, spec types.ResourceConfigSpec) | tier2 | vsphere-general | `vmware_resource_pool_create` | fail-safe default (no pattern matched) |
| ResourcePool | CreateVApp(name string, resSpec types.ResourceConfigSpec, configSpec types.VAppConfigSpec, folder *Folder) | tier2 | vsphere-general | `vmware_resource_pool_create_vapp` | fail-safe default (no pattern matched) |
| ResourcePool | Destroy() | tier1 | vsphere-general | `vmware_resource_pool_destroy` | name matches tier1 pattern |
| ResourcePool | DestroyChildren() | tier1 | vsphere-general | `vmware_resource_pool_destroy_children` | name matches tier1 pattern |
| ResourcePool | ImportVApp(spec types.BaseImportSpec, folder *Folder, host *HostSystem) | tier2 | vsphere-general | `vmware_resource_pool_import_vapp` | fail-safe default (no pattern matched) |
| ResourcePool | Owner() | tier2 | vsphere-general | `vmware_resource_pool_owner` | fail-safe default (no pattern matched) |
| ResourcePool | UpdateConfig(name string, config *types.ResourceConfigSpec) | tier2 | vsphere-general | `vmware_resource_pool_update_config` | fail-safe default (no pattern matched) |
| SearchIndex | FindAllByDnsName(dc *Datacenter, dnsName string, vmSearch bool) | — | vsphere-general | `vmware_search_find_all_by_dns_name` | name matches read-only pattern |
| SearchIndex | FindAllByIp(dc *Datacenter, ip string, vmSearch bool) | — | vsphere-general | `vmware_search_find_all_by_ip` | name matches read-only pattern |
| SearchIndex | FindAllByUuid(dc *Datacenter, uuid string, vmSearch bool, instanceUuid *bool) | — | vsphere-general | `vmware_search_find_all_by_uuid` | name matches read-only pattern |
| SearchIndex | FindByDatastorePath(dc *Datacenter, path string) | — | vsphere-general | `vmware_search_find_by_datastore_path` | name matches read-only pattern |
| SearchIndex | FindByDnsName(dc *Datacenter, dnsName string, vmSearch bool) | — | vsphere-general | `vmware_search_find_by_dns_name` | name matches read-only pattern |
| SearchIndex | FindByInventoryPath(path string) | — | vsphere-general | `vmware_search_find_by_inventory_path` | name matches read-only pattern |
| SearchIndex | FindByIp(dc *Datacenter, ip string, vmSearch bool) | — | vsphere-general | `vmware_search_find_by_ip` | name matches read-only pattern |
| SearchIndex | FindByUuid(dc *Datacenter, uuid string, vmSearch bool, instanceUuid *bool) | — | vsphere-general | `vmware_search_find_by_uuid` | name matches read-only pattern |
| SearchIndex | FindChild(entity Reference, name string) | — | vsphere-general | `vmware_search_find_child` | name matches read-only pattern |
| StorageResourceManager | ApplyStorageDrsRecommendation(key []string) | tier2 | vsphere-general | `vmware_storage_apply_storage_drs_recommendation` | fail-safe default (no pattern matched) |
| StorageResourceManager | ApplyStorageDrsRecommendationToPod(pod *StoragePod, key string) | tier2 | vsphere-general | `vmware_storage_apply_storage_drs_recommendation_to_pod` | fail-safe default (no pattern matched) |
| StorageResourceManager | CancelStorageDrsRecommendation(key []string) | tier2 | vsphere-general | `vmware_storage_cancel_storage_drs_recommendation` | fail-safe default (no pattern matched) |
| StorageResourceManager | ConfigureDatastoreIORM(datastore *Datastore, spec types.StorageIORMConfigSpec, key string) | tier2 | vsphere-general | `vmware_storage_configure_datastore_iorm` | fail-safe default (no pattern matched) |
| StorageResourceManager | ConfigureStorageDrsForPod(pod *StoragePod, spec types.StorageDrsConfigSpec, modify bool) | tier2 | vsphere-general | `vmware_storage_configure_storage_drs_for_pod` | fail-safe default (no pattern matched) |
| StorageResourceManager | QueryDatastorePerformanceSummary(datastore *Datastore) | — | vsphere-general | `vmware_storage_query_datastore_performance_summary` | name matches read-only pattern |
| StorageResourceManager | QueryIORMConfigOption(host *HostSystem) | — | vsphere-general | `vmware_storage_query_iormconfig_option` | name matches read-only pattern |
| StorageResourceManager | RecommendDatastores(storageSpec types.StoragePlacementSpec) | tier2 | vsphere-general | `vmware_storage_recommend_datastores` | fail-safe default (no pattern matched) |
| StorageResourceManager | RefreshStorageDrsRecommendation(pod *StoragePod) | tier2 | vsphere-general | `vmware_storage_refresh_storage_drs_recommendation` | fail-safe default (no pattern matched) |
| Task | Cancel() | tier2 | vsphere-general | `vmware_task_cancel` | fail-safe default (no pattern matched) |
| Task | SetDescription(description types.LocalizableMessage) | tier2 | vsphere-general | `vmware_task_set_description` | fail-safe default (no pattern matched) |
| Task | SetState(state types.TaskInfoState, result types.AnyType, fault *types.LocalizedMethodFault) | tier2 | vsphere-general | `vmware_task_set_state` | fail-safe default (no pattern matched) |
| Task | UpdateProgress(percentDone int) | tier2 | vsphere-general | `vmware_task_update_progress` | fail-safe default (no pattern matched) |
| Task | Wait() | — | vsphere-general | `vmware_task_wait` | name matches read-only pattern |
| Task | WaitEx() | — | vsphere-general | `vmware_task_wait_ex` | name matches read-only pattern |
| Task | WaitForResult(s ...progress.Sinker) | — | vsphere-general | `vmware_task_wait_for_result` | name matches read-only pattern |
| Task | WaitForResultEx(s ...progress.Sinker) | — | vsphere-general | `vmware_task_wait_for_result_ex` | name matches read-only pattern |
| TenantManager | MarkServiceProviderEntities(entities []types.ManagedObjectReference) | tier2 | vcenter-only | `vmware_tenant_mark_service_provider_entities` | fail-safe default (no pattern matched) |
| TenantManager | RetrieveServiceProviderEntities() | — | vcenter-only | `vmware_tenant_retrieve_service_provider_entities` | name matches read-only pattern |
| TenantManager | UnmarkServiceProviderEntities(entities []types.ManagedObjectReference) | tier2 | vcenter-only | `vmware_tenant_unmark_service_provider_entities` | fail-safe default (no pattern matched) |
| VirtualApp | Clone(name string, target types.ManagedObjectReference, spec types.VAppCloneSpec) | tier2 | vsphere-general | `vmware_vapp_clone` | fail-safe default (no pattern matched) |
| VirtualApp | CreateChildVM(config types.VirtualMachineConfigSpec, host *HostSystem) | tier2 | vsphere-general | `vmware_vapp_create_child_vm` | fail-safe default (no pattern matched) |
| VirtualApp | PowerOff(force bool) | tier2 | vsphere-general | `vmware_vapp_power_off` | name matches tier2 pattern |
| VirtualApp | PowerOn() | tier2 | vsphere-general | `vmware_vapp_power_on` | fail-safe default (no pattern matched) |
| VirtualApp | Suspend() | tier2 | vsphere-general | `vmware_vapp_suspend` | fail-safe default (no pattern matched) |
| VirtualApp | UpdateConfig(spec types.VAppConfigSpec) | tier2 | vsphere-general | `vmware_vapp_update_config` | fail-safe default (no pattern matched) |
| VirtualDiskManager | CopyVirtualDisk(sourceName string, sourceDatacenter *Datacenter, destName string, destDatacenter *Datacenter, destSpec types.BaseVirtualDiskSpec, force bool) | tier2 | vsphere-general | `vmware_virtual_disk_copy_virtual_disk` | fail-safe default (no pattern matched) |
| VirtualDiskManager | CreateChildDisk(parent string, pdc *Datacenter, name string, dc *Datacenter, linked bool) | tier2 | vsphere-general | `vmware_virtual_disk_create_child_disk` | fail-safe default (no pattern matched) |
| VirtualDiskManager | CreateVirtualDisk(name string, datacenter *Datacenter, spec types.BaseVirtualDiskSpec) | tier2 | vsphere-general | `vmware_virtual_disk_create_virtual_disk` | fail-safe default (no pattern matched) |
| VirtualDiskManager | DeleteVirtualDisk(name string, dc *Datacenter) | tier1 | vsphere-general | `vmware_virtual_disk_delete_virtual_disk` | name matches tier1 pattern |
| VirtualDiskManager | ExtendVirtualDisk(name string, datacenter *Datacenter, capacityKb int64, eagerZero *bool) | tier2 | vsphere-general | `vmware_virtual_disk_extend_virtual_disk` | fail-safe default (no pattern matched) |
| VirtualDiskManager | InflateVirtualDisk(name string, dc *Datacenter) | tier2 | vsphere-general | `vmware_virtual_disk_inflate_virtual_disk` | fail-safe default (no pattern matched) |
| VirtualDiskManager | MoveVirtualDisk(sourceName string, sourceDatacenter *Datacenter, destName string, destDatacenter *Datacenter, force bool) | tier2 | vsphere-general | `vmware_virtual_disk_move_virtual_disk` | fail-safe default (no pattern matched) |
| VirtualDiskManager | QueryVirtualDiskInfo(name string, dc *Datacenter, includeParents bool) | — | vsphere-general | `vmware_virtual_disk_query_virtual_disk_info` | name matches read-only pattern |
| VirtualDiskManager | QueryVirtualDiskUuid(name string, dc *Datacenter) | — | vsphere-general | `vmware_virtual_disk_query_virtual_disk_uuid` | name matches read-only pattern |
| VirtualDiskManager | SetVirtualDiskUuid(name string, dc *Datacenter, uuid string) | tier2 | vsphere-general | `vmware_virtual_disk_set_virtual_disk_uuid` | fail-safe default (no pattern matched) |
| VirtualDiskManager | ShrinkVirtualDisk(name string, dc *Datacenter, copy *bool) | tier2 | vsphere-general | `vmware_virtual_disk_shrink_virtual_disk` | fail-safe default (no pattern matched) |
| VirtualMachine | AcquireTicket(kind string) | tier2 | vsphere-general | `vmware_vm_acquire_ticket` | fail-safe default (no pattern matched) |
| VirtualMachine | AddDevice(device ...types.BaseVirtualDevice) | tier2 | vsphere-general | `vmware_vm_add_device` | fail-safe default (no pattern matched) |
| VirtualMachine | AddDeviceWithProfile(profile []types.BaseVirtualMachineProfileSpec, device ...types.BaseVirtualDevice) | tier2 | vsphere-general | `vmware_vm_add_device_with_profile` | fail-safe default (no pattern matched) |
| VirtualMachine | Answer(id string, answer string) | tier2 | vsphere-general | `vmware_vm_answer` | fail-safe default (no pattern matched) |
| VirtualMachine | AttachDisk(id string, datastore *Datastore, controllerKey int32, unitNumber *int32) | tier2 | vsphere-general | `vmware_vm_attach_disk` | fail-safe default (no pattern matched) |
| VirtualMachine | BootOptions() | tier2 | vsphere-general | `vmware_vm_boot_options` | fail-safe default (no pattern matched) |
| VirtualMachine | Clone(folder *Folder, name string, config types.VirtualMachineCloneSpec) | tier2 | vsphere-general | `vmware_vm_clone` | fail-safe default (no pattern matched) |
| VirtualMachine | CreateSnapshot(name string, description string, memory bool, quiesce bool) | tier2 | vsphere-general | `vmware_vm_create_snapshot` | fail-safe default (no pattern matched) |
| VirtualMachine | CreateSnapshotEx(name string, description string, memory bool, quiesceSpec types.BaseVirtualMachineGuestQuiesceSpec) | tier2 | vsphere-general | `vmware_vm_create_snapshot_ex` | fail-safe default (no pattern matched) |
| VirtualMachine | Customize(spec types.CustomizationSpec) | tier2 | vsphere-general | `vmware_vm_customize` | fail-safe default (no pattern matched) |
| VirtualMachine | DetachDisk(id string) | tier1 | vsphere-general | `vmware_vm_detach_disk` | name matches tier1 pattern |
| VirtualMachine | Device() | tier2 | vsphere-general | `vmware_vm_device` | fail-safe default (no pattern matched) |
| VirtualMachine | EditDevice(device ...types.BaseVirtualDevice) | tier2 | vsphere-general | `vmware_vm_edit_device` | fail-safe default (no pattern matched) |
| VirtualMachine | EditDeviceWithProfile(profile []types.BaseVirtualMachineProfileSpec, device ...types.BaseVirtualDevice) | tier2 | vsphere-general | `vmware_vm_edit_device_with_profile` | fail-safe default (no pattern matched) |
| VirtualMachine | EnvironmentBrowser() | tier2 | vsphere-general | `vmware_vm_environment_browser` | fail-safe default (no pattern matched) |
| VirtualMachine | Export() | tier2 | vsphere-general | `vmware_vm_export` | fail-safe default (no pattern matched) |
| VirtualMachine | ExportSnapshot(snapshot *types.ManagedObjectReference) | tier2 | vsphere-general | `vmware_vm_export_snapshot` | fail-safe default (no pattern matched) |
| VirtualMachine | FindSnapshot(name string) | — | vsphere-general | `vmware_vm_find_snapshot` | name matches read-only pattern |
| VirtualMachine | HostSystem() | tier2 | vsphere-general | `vmware_vm_host_system` | fail-safe default (no pattern matched) |
| VirtualMachine | InstantClone(config types.VirtualMachineInstantCloneSpec) | tier2 | vsphere-general | `vmware_vm_instant_clone` | fail-safe default (no pattern matched) |
| VirtualMachine | IsTemplate() | — | vsphere-general | `vmware_vm_is_template` | name matches read-only pattern |
| VirtualMachine | IsToolsRunning() | — | vsphere-general | `vmware_vm_is_tools_running` | name matches read-only pattern |
| VirtualMachine | MarkAsTemplate() | tier2 | vsphere-general | `vmware_vm_mark_as_template` | fail-safe default (no pattern matched) |
| VirtualMachine | MarkAsVirtualMachine(pool ResourcePool, host *HostSystem) | tier2 | vsphere-general | `vmware_vm_mark_as_virtual_machine` | fail-safe default (no pattern matched) |
| VirtualMachine | Migrate(pool *ResourcePool, host *HostSystem, priority types.VirtualMachineMovePriority, state types.VirtualMachinePowerState) | tier2 | vsphere-general | `vmware_vm_migrate` | fail-safe default (no pattern matched) |
| VirtualMachine | MountToolsInstaller() | tier2 | vsphere-general | `vmware_vm_mount_tools_installer` | fail-safe default (no pattern matched) |
| VirtualMachine | PowerState() | tier2 | vsphere-general | `vmware_vm_power_state` | fail-safe default (no pattern matched) |
| VirtualMachine | PromoteDisks(unlink bool, disks []types.VirtualDisk) | tier2 | vsphere-general | `vmware_vm_promote_disks` | fail-safe default (no pattern matched) |
| VirtualMachine | PutUsbScanCodes(spec types.UsbScanCodeSpec) | tier2 | vsphere-general | `vmware_vm_put_usb_scan_codes` | fail-safe default (no pattern matched) |
| VirtualMachine | QueryChangedDiskAreas(baseSnapshot *types.ManagedObjectReference, curSnapshot *types.ManagedObjectReference, disk *types.VirtualDisk, offset int64) | — | vsphere-general | `vmware_vm_query_changed_disk_areas` | name matches read-only pattern |
| VirtualMachine | RebootGuest() | tier2 | vsphere-general | `vmware_vm_reboot_guest` | name matches tier2 pattern |
| VirtualMachine | RefreshStorageInfo() | tier2 | vsphere-general | `vmware_vm_refresh_storage_info` | fail-safe default (no pattern matched) |
| VirtualMachine | Relocate(config types.VirtualMachineRelocateSpec, priority types.VirtualMachineMovePriority) | tier2 | vsphere-general | `vmware_vm_relocate` | fail-safe default (no pattern matched) |
| VirtualMachine | RemoveAllSnapshot(consolidate *bool) | tier1 | vsphere-general | `vmware_vm_remove_all_snapshot` | name matches tier1 pattern |
| VirtualMachine | RemoveDevice(keepFiles bool, device ...types.BaseVirtualDevice) | tier1 | vsphere-general | `vmware_vm_remove_device` | name matches tier1 pattern |
| VirtualMachine | RemoveSnapshot(name string, removeChildren bool, consolidate *bool) | tier1 | vsphere-general | `vmware_vm_remove_snapshot` | name matches tier1 pattern |
| VirtualMachine | ResourcePool() | tier2 | vsphere-general | `vmware_vm_resource_pool` | fail-safe default (no pattern matched) |
| VirtualMachine | RevertToCurrentSnapshot(suppressPowerOn bool) | tier1 | vsphere-general | `vmware_vm_revert_to_current_snapshot` | name matches tier1 pattern |
| VirtualMachine | RevertToSnapshot(name string, suppressPowerOn bool) | tier1 | vsphere-general | `vmware_vm_revert_to_snapshot` | name matches tier1 pattern |
| VirtualMachine | SetBootOptions(options *types.VirtualMachineBootOptions) | tier2 | vsphere-general | `vmware_vm_set_boot_options` | fail-safe default (no pattern matched) |
| VirtualMachine | ShutdownGuest() | tier2 | vsphere-general | `vmware_vm_shutdown_guest` | name matches tier2 pattern |
| VirtualMachine | StandbyGuest() | tier2 | vsphere-general | `vmware_vm_standby_guest` | fail-safe default (no pattern matched) |
| VirtualMachine | UUID() | tier2 | vsphere-general | `vmware_vm_uuid` | fail-safe default (no pattern matched) |
| VirtualMachine | UnmountToolsInstaller() | tier1 | vsphere-general | `vmware_vm_unmount_tools_installer` | name matches tier1 pattern |
| VirtualMachine | Unregister() | tier1 | vsphere-general | `vmware_vm_unregister` | name matches tier1 pattern |
| VirtualMachine | UpgradeTools(options string) | tier2 | vsphere-general | `vmware_vm_upgrade_tools` | fail-safe default (no pattern matched) |
| VirtualMachine | UpgradeVM(version string) | tier2 | vsphere-general | `vmware_vm_upgrade_vm` | fail-safe default (no pattern matched) |
| VirtualMachine | WaitForIP(v4 ...bool) | — | vsphere-general | `vmware_vm_wait_for_ip` | name matches read-only pattern |
| VirtualMachine | WaitForNetIP(v4 bool, device ...string) | — | vsphere-general | `vmware_vm_wait_for_net_ip` | name matches read-only pattern |
| VirtualMachine | WaitForPowerState(state types.VirtualMachinePowerState) | — | vsphere-general | `vmware_vm_wait_for_power_state` | name matches read-only pattern |
| VmCompatibilityChecker | CheckCompatibility(vm types.ManagedObjectReference, host *types.ManagedObjectReference, pool *types.ManagedObjectReference, testTypes ...types.CheckTestType) | tier2 | vsphere-general | `vmware_vm_compatibility_checker_check_compatibility` | fail-safe default (no pattern matched) |
| VmCompatibilityChecker | CheckVmConfig(spec types.VirtualMachineConfigSpec, vm *types.ManagedObjectReference, host *types.ManagedObjectReference, pool *types.ManagedObjectReference, testTypes ...types.CheckTestType) | tier2 | vsphere-general | `vmware_vm_compatibility_checker_check_vm_config` | fail-safe default (no pattern matched) |
| VmProvisioningChecker | CheckRelocate(vm types.ManagedObjectReference, spec types.VirtualMachineRelocateSpec, testTypes ...types.CheckTestType) | tier2 | vsphere-general | `vmware_vm_provisioning_checker_check_relocate` | fail-safe default (no pattern matched) |

## vapi/appliance/access/consolecli (2 métodos)

| Receiver | Método | Tier | Modo | Tool proposta | Regra |
|---|---|---|---|---|---|
| Manager | Get() | — | vcenter-only | `vmware_appliance_access_consolecli_get` | name matches read-only pattern |
| Manager | Set(inp Access) | tier2 | vcenter-only | `vmware_appliance_access_consolecli_set` | fail-safe default (no pattern matched) |

## vapi/appliance/access/dcui (2 métodos)

| Receiver | Método | Tier | Modo | Tool proposta | Regra |
|---|---|---|---|---|---|
| Manager | Get() | — | vcenter-only | `vmware_appliance_access_dcui_get` | name matches read-only pattern |
| Manager | Set(inp Access) | tier2 | vcenter-only | `vmware_appliance_access_dcui_set` | fail-safe default (no pattern matched) |

## vapi/appliance/access/shell (2 métodos)

| Receiver | Método | Tier | Modo | Tool proposta | Regra |
|---|---|---|---|---|---|
| Manager | Get() | — | vcenter-only | `vmware_appliance_access_shell_get` | name matches read-only pattern |
| Manager | Set(inp Access) | tier2 | vcenter-only | `vmware_appliance_access_shell_set` | fail-safe default (no pattern matched) |

## vapi/appliance/access/ssh (2 métodos)

| Receiver | Método | Tier | Modo | Tool proposta | Regra |
|---|---|---|---|---|---|
| Manager | Get() | — | vcenter-only | `vmware_appliance_access_ssh_get` | name matches read-only pattern |
| Manager | Set(inp Access) | tier2 | vcenter-only | `vmware_appliance_access_ssh_set` | fail-safe default (no pattern matched) |

## vapi/appliance/logging (1 métodos)

| Receiver | Método | Tier | Modo | Tool proposta | Regra |
|---|---|---|---|---|---|
| Manager | Forwarding() | tier2 | vcenter-only | `vmware_appliance_logging_forwarding` | fail-safe default (no pattern matched) |

## vapi/appliance/networking (2 métodos)

| Receiver | Método | Tier | Modo | Tool proposta | Regra |
|---|---|---|---|---|---|
| Manager | NoProxy() | tier2 | vcenter-only | `vmware_appliance_networking_no_proxy` | fail-safe default (no pattern matched) |
| Manager | ProxyList() | tier2 | vcenter-only | `vmware_appliance_networking_proxy_list` | fail-safe default (no pattern matched) |

## vapi/appliance/shutdown (4 métodos)

| Receiver | Método | Tier | Modo | Tool proposta | Regra |
|---|---|---|---|---|---|
| Manager | Cancel() | tier2 | vcenter-only | `vmware_appliance_shutdown_cancel` | fail-safe default (no pattern matched) |
| Manager | Get() | — | vcenter-only | `vmware_appliance_shutdown_get` | name matches read-only pattern |
| Manager | PowerOff(reason string, delay int) | tier2 | vcenter-only | `vmware_appliance_shutdown_power_off` | name matches tier2 pattern |
| Manager | Reboot(reason string, delay int) | tier2 | vcenter-only | `vmware_appliance_shutdown_reboot` | name matches tier2 pattern |

## vapi/authentication (1 métodos)

| Receiver | Método | Tier | Modo | Tool proposta | Regra |
|---|---|---|---|---|---|
| Manager | Issue(token TokenIssueSpec) | — | vcenter-only | `vmware_authentication_issue` | name matches read-only pattern |

## vapi/cis/tasks (4 métodos)

| Receiver | Método | Tier | Modo | Tool proposta | Regra |
|---|---|---|---|---|---|
| Manager | Get(taskId string) | — | vcenter-only | `vmware_cis_tasks_get` | name matches read-only pattern |
| Manager | WaitForCompletion(taskId string) | — | vcenter-only | `vmware_cis_tasks_wait_for_completion` | name matches read-only pattern |
| Manager | WaitForRunningOrError(taskId string) | — | vcenter-only | `vmware_cis_tasks_wait_for_running_or_error` | name matches read-only pattern |
| Manager | WaitForRunningOrTerminalState(taskId string) | — | vcenter-only | `vmware_cis_tasks_wait_for_running_or_terminal_state` | name matches read-only pattern |

## vapi/cluster (6 métodos)

| Receiver | Método | Tier | Modo | Tool proposta | Regra |
|---|---|---|---|---|---|
| Manager | AddModuleMembers(id string, vms ...mo.Reference) | tier2 | vcenter-only | `vmware_cluster_add_module_members` | fail-safe default (no pattern matched) |
| Manager | CreateModule(ref mo.Reference) | tier2 | vcenter-only | `vmware_cluster_create_module` | fail-safe default (no pattern matched) |
| Manager | DeleteModule(id string) | tier1 | vcenter-only | `vmware_cluster_delete_module` | name matches tier1 pattern |
| Manager | ListModuleMembers(id string) | — | vcenter-only | `vmware_cluster_list_module_members` | name matches read-only pattern |
| Manager | ListModules() | — | vcenter-only | `vmware_cluster_list_modules` | name matches read-only pattern |
| Manager | RemoveModuleMembers(id string, vms ...mo.Reference) | tier1 | vcenter-only | `vmware_cluster_remove_module_members` | name matches tier1 pattern |

## vapi/crypto (4 métodos)

| Receiver | Método | Tier | Modo | Tool proposta | Regra |
|---|---|---|---|---|---|
| Manager | KmsProviderCreate(spec KmsProviderCreateSpec) | tier2 | vcenter-only | `vmware_crypto_kms_provider_create` | fail-safe default (no pattern matched) |
| Manager | KmsProviderDelete(provider string) | tier1 | vcenter-only | `vmware_crypto_kms_provider_delete` | tier1 verb found mid-name, not prefix (e.g. ForceDeleteX) |
| Manager | KmsProviderExport(spec KmsProviderExportSpec) | tier2 | vcenter-only | `vmware_crypto_kms_provider_export` | fail-safe default (no pattern matched) |
| Manager | KmsProviderExportRequest(export *KmsProviderExportLocation) | tier2 | vcenter-only | `vmware_crypto_kms_provider_export_request` | fail-safe default (no pattern matched) |

## vapi/esx/settings/clusters/vms (17 métodos)

| Receiver | Método | Tier | Modo | Tool proposta | Regra |
|---|---|---|---|---|---|
| Manager | Apply(cluster types.ManagedObjectReference, applySpec *ApplySpec) | tier2 | vcenter-only | `vmware_esx_settings_clusters_vms_apply` | fail-safe default (no pattern matched) |
| Manager | ApplyWaitForCompletion(taskId string) | tier2 | vcenter-only | `vmware_esx_settings_clusters_vms_apply_wait_for_completion` | fail-safe default (no pattern matched) |
| Manager | CheckCompliance(cluster types.ManagedObjectReference, filterSpec *CheckComplianceFilterSpec) | tier2 | vcenter-only | `vmware_esx_settings_clusters_vms_check_compliance` | fail-safe default (no pattern matched) |
| Manager | Delete(cluster types.ManagedObjectReference, solution string) | tier1 | vcenter-only | `vmware_esx_settings_clusters_vms_delete` | name matches tier1 pattern |
| Manager | DeleteSolutionOnly(cluster types.ManagedObjectReference, solution string) | tier1 | vcenter-only | `vmware_esx_settings_clusters_vms_delete_solution_only` | name matches tier1 pattern |
| Manager | Enable(cluster types.ManagedObjectReference, solution string, spec *EnableSpec) | tier2 | vcenter-only | `vmware_esx_settings_clusters_vms_enable` | fail-safe default (no pattern matched) |
| Manager | EnableAsync(cluster types.ManagedObjectReference, solution string, spec *EnableSpec) | tier2 | vcenter-only | `vmware_esx_settings_clusters_vms_enable_async` | fail-safe default (no pattern matched) |
| Manager | Get(cluster types.ManagedObjectReference, solution string) | — | vcenter-only | `vmware_esx_settings_clusters_vms_get` | name matches read-only pattern |
| Manager | List(cluster types.ManagedObjectReference) | — | vcenter-only | `vmware_esx_settings_clusters_vms_list` | name matches read-only pattern |
| Manager | ListHooks(cluster types.ManagedObjectReference, solution string) | — | vcenter-only | `vmware_esx_settings_clusters_vms_list_hooks` | name matches read-only pattern |
| Manager | MarkAsProcessed(cluster types.ManagedObjectReference, spec *ProcessedHookSpec) | tier2 | vcenter-only | `vmware_esx_settings_clusters_vms_mark_as_processed` | fail-safe default (no pattern matched) |
| Manager | MultiSourceEnable(cluster types.ManagedObjectReference, solution string, spec *MultiSourceEnableSpec) | tier2 | vcenter-only | `vmware_esx_settings_clusters_vms_multi_source_enable` | fail-safe default (no pattern matched) |
| Manager | MultiSourceEnableAsync(cluster types.ManagedObjectReference, solution string, spec *MultiSourceEnableSpec) | tier2 | vcenter-only | `vmware_esx_settings_clusters_vms_multi_source_enable_async` | fail-safe default (no pattern matched) |
| Manager | ProcessDynamicUpdate(cluster types.ManagedObjectReference, spec *DynamicUpdateSpec) | tier2 | vcenter-only | `vmware_esx_settings_clusters_vms_process_dynamic_update` | fail-safe default (no pattern matched) |
| Manager | Set(cluster types.ManagedObjectReference, solution string, spec *SolutionSpec) | tier2 | vcenter-only | `vmware_esx_settings_clusters_vms_set` | fail-safe default (no pattern matched) |
| Manager | Transition(cluster types.ManagedObjectReference, solution string, spec *TransitionSpec) | tier2 | vcenter-only | `vmware_esx_settings_clusters_vms_transition` | fail-safe default (no pattern matched) |
| Manager | TransitionAsync(cluster types.ManagedObjectReference, solution string, spec *TransitionSpec) | tier2 | vcenter-only | `vmware_esx_settings_clusters_vms_transition_async` | fail-safe default (no pattern matched) |

## vapi/library (67 métodos)

| Receiver | Método | Tier | Modo | Tool proposta | Regra |
|---|---|---|---|---|---|
| Manager | AddLibraryItemFile(sessionID string, updateFile UpdateFile) | tier2 | vcenter-only | `vmware_library_add_library_item_file` | fail-safe default (no pattern matched) |
| Manager | AddLibraryItemFileFromURI(sessionID string, name string, uri string, checksum ...Checksum) | tier2 | vcenter-only | `vmware_library_add_library_item_file_from_uri` | fail-safe default (no pattern matched) |
| Manager | AddLibraryUsage(libraryID string, addUsage AddUsage) | tier2 | vcenter-only | `vmware_library_add_library_usage` | fail-safe default (no pattern matched) |
| Manager | CancelLibraryItemDownloadSession(id string) | tier2 | vcenter-only | `vmware_library_cancel_library_item_download_session` | fail-safe default (no pattern matched) |
| Manager | CancelLibraryItemUpdateSession(id string) | tier2 | vcenter-only | `vmware_library_cancel_library_item_update_session` | fail-safe default (no pattern matched) |
| Manager | CompleteLibraryItemUpdateSession(id string) | tier2 | vcenter-only | `vmware_library_complete_library_item_update_session` | fail-safe default (no pattern matched) |
| Manager | CopyLibraryItem(src *Item, dst Item) | tier2 | vcenter-only | `vmware_library_copy_library_item` | fail-safe default (no pattern matched) |
| Manager | CreateLibrary(library Library) | tier2 | vcenter-only | `vmware_library_create_library` | fail-safe default (no pattern matched) |
| Manager | CreateLibraryItem(item Item) | tier2 | vcenter-only | `vmware_library_create_library_item` | fail-safe default (no pattern matched) |
| Manager | CreateLibraryItemDownloadSession(session Session) | tier2 | vcenter-only | `vmware_library_create_library_item_download_session` | fail-safe default (no pattern matched) |
| Manager | CreateLibraryItemUpdateSession(session Session) | tier2 | vcenter-only | `vmware_library_create_library_item_update_session` | fail-safe default (no pattern matched) |
| Manager | CreateSubscriber(library *Library, s SubscriberLibrary) | tier2 | vcenter-only | `vmware_library_create_subscriber` | fail-safe default (no pattern matched) |
| Manager | CreateTrustedCertificate(cert string) | tier2 | vcenter-only | `vmware_library_create_trusted_certificate` | fail-safe default (no pattern matched) |
| Manager | DefaultOvfSecurityPolicy() | tier2 | vcenter-only | `vmware_library_default_ovf_security_policy` | fail-safe default (no pattern matched) |
| Manager | DeleteLibrary(library *Library) | tier1 | vcenter-only | `vmware_library_delete_library` | name matches tier1 pattern |
| Manager | DeleteLibraryItem(item *Item) | tier1 | vcenter-only | `vmware_library_delete_library_item` | name matches tier1 pattern |
| Manager | DeleteLibraryItemDownloadSession(id string) | tier1 | vcenter-only | `vmware_library_delete_library_item_download_session` | name matches tier1 pattern |
| Manager | DeleteLibraryItemUpdateSession(id string) | tier1 | vcenter-only | `vmware_library_delete_library_item_update_session` | name matches tier1 pattern |
| Manager | DeleteSubscriber(library *Library, subscriber string) | tier1 | vcenter-only | `vmware_library_delete_subscriber` | name matches tier1 pattern |
| Manager | DeleteTrustedCertificate(id string) | tier1 | vcenter-only | `vmware_library_delete_trusted_certificate` | name matches tier1 pattern |
| Manager | EvictSubscribedLibrary(library *Library) | tier2 | vcenter-only | `vmware_library_evict_subscribed_library` | fail-safe default (no pattern matched) |
| Manager | EvictSubscribedLibraryItem(item *Item) | tier2 | vcenter-only | `vmware_library_evict_subscribed_library_item` | fail-safe default (no pattern matched) |
| Manager | FailLibraryItemDownloadSession(id string) | tier2 | vcenter-only | `vmware_library_fail_library_item_download_session` | fail-safe default (no pattern matched) |
| Manager | FailLibraryItemUpdateSession(id string) | tier2 | vcenter-only | `vmware_library_fail_library_item_update_session` | fail-safe default (no pattern matched) |
| Manager | FindLibrary(search Find) | — | vcenter-only | `vmware_library_find_library` | name matches read-only pattern |
| Manager | FindLibraryItems(search FindItem) | — | vcenter-only | `vmware_library_find_library_items` | name matches read-only pattern |
| Manager | ForceDeleteLibrary(library *Library) | tier1 | vcenter-only | `vmware_library_force_delete_library` | tier1 verb found mid-name, not prefix (e.g. ForceDeleteX) |
| Manager | GetLibraries() | — | vcenter-only | `vmware_library_get_libraries` | name matches read-only pattern |
| Manager | GetLibraryByID(id string) | — | vcenter-only | `vmware_library_get_library_by_id` | name matches read-only pattern |
| Manager | GetLibraryByName(name string) | — | vcenter-only | `vmware_library_get_library_by_name` | name matches read-only pattern |
| Manager | GetLibraryItem(id string) | — | vcenter-only | `vmware_library_get_library_item` | name matches read-only pattern |
| Manager | GetLibraryItemDownloadSession(id string) | — | vcenter-only | `vmware_library_get_library_item_download_session` | name matches read-only pattern |
| Manager | GetLibraryItemDownloadSessionFile(sessionID string, name string) | — | vcenter-only | `vmware_library_get_library_item_download_session_file` | name matches read-only pattern |
| Manager | GetLibraryItemFile(id string, fileName string) | — | vcenter-only | `vmware_library_get_library_item_file` | name matches read-only pattern |
| Manager | GetLibraryItemStorage(id string, fileName string) | — | vcenter-only | `vmware_library_get_library_item_storage` | name matches read-only pattern |
| Manager | GetLibraryItemUpdateSession(id string) | — | vcenter-only | `vmware_library_get_library_item_update_session` | name matches read-only pattern |
| Manager | GetLibraryItemUpdateSessionFile(sessionID string, fileName string) | — | vcenter-only | `vmware_library_get_library_item_update_session_file` | name matches read-only pattern |
| Manager | GetLibraryItems(libraryID string) | — | vcenter-only | `vmware_library_get_library_items` | name matches read-only pattern |
| Manager | GetLibraryUsage(libraryID string, usageID string) | — | vcenter-only | `vmware_library_get_library_usage` | name matches read-only pattern |
| Manager | GetSubscriber(library *Library, subscriber string) | — | vcenter-only | `vmware_library_get_subscriber` | name matches read-only pattern |
| Manager | GetTrustedCertificate(id string) | — | vcenter-only | `vmware_library_get_trusted_certificate` | name matches read-only pattern |
| Manager | KeepAliveLibraryItemDownloadSession(id string) | tier2 | vcenter-only | `vmware_library_keep_alive_library_item_download_session` | fail-safe default (no pattern matched) |
| Manager | KeepAliveLibraryItemUpdateSession(id string) | tier2 | vcenter-only | `vmware_library_keep_alive_library_item_update_session` | fail-safe default (no pattern matched) |
| Manager | ListLibraries() | — | vcenter-only | `vmware_library_list_libraries` | name matches read-only pattern |
| Manager | ListLibraryItemDownloadSession() | — | vcenter-only | `vmware_library_list_library_item_download_session` | name matches read-only pattern |
| Manager | ListLibraryItemDownloadSessionFile(sessionID string) | — | vcenter-only | `vmware_library_list_library_item_download_session_file` | name matches read-only pattern |
| Manager | ListLibraryItemFiles(id string) | — | vcenter-only | `vmware_library_list_library_item_files` | name matches read-only pattern |
| Manager | ListLibraryItemStorage(id string) | — | vcenter-only | `vmware_library_list_library_item_storage` | name matches read-only pattern |
| Manager | ListLibraryItemUpdateSession() | — | vcenter-only | `vmware_library_list_library_item_update_session` | name matches read-only pattern |
| Manager | ListLibraryItemUpdateSessionFile(sessionID string) | — | vcenter-only | `vmware_library_list_library_item_update_session_file` | name matches read-only pattern |
| Manager | ListLibraryItems(id string) | — | vcenter-only | `vmware_library_list_library_items` | name matches read-only pattern |
| Manager | ListLibraryUsage(libraryID string) | — | vcenter-only | `vmware_library_list_library_usage` | name matches read-only pattern |
| Manager | ListSecurityPolicies() | — | vcenter-only | `vmware_library_list_security_policies` | name matches read-only pattern |
| Manager | ListSubscribers(library *Library) | — | vcenter-only | `vmware_library_list_subscribers` | name matches read-only pattern |
| Manager | ListTrustedCertificates() | — | vcenter-only | `vmware_library_list_trusted_certificates` | name matches read-only pattern |
| Manager | PrepareLibraryItemDownloadSessionFile(sessionID string, name string) | tier2 | vcenter-only | `vmware_library_prepare_library_item_download_session_file` | fail-safe default (no pattern matched) |
| Manager | ProbeTransferEndpoint(endpoint TransferEndpoint) | tier2 | vcenter-only | `vmware_library_probe_transfer_endpoint` | fail-safe default (no pattern matched) |
| Manager | PublishLibrary(library *Library, subscriptions []string) | tier2 | vcenter-only | `vmware_library_publish_library` | fail-safe default (no pattern matched) |
| Manager | PublishLibraryItem(item *Item, force bool, subscriptions []string) | tier2 | vcenter-only | `vmware_library_publish_library_item` | fail-safe default (no pattern matched) |
| Manager | RemoveLibraryItemUpdateSessionFile(sessionID string, fileName string) | tier1 | vcenter-only | `vmware_library_remove_library_item_update_session_file` | name matches tier1 pattern |
| Manager | RemoveLibraryUsage(libraryID string, usageID string) | tier1 | vcenter-only | `vmware_library_remove_library_usage` | name matches tier1 pattern |
| Manager | SyncLibrary(library *Library) | tier2 | vcenter-only | `vmware_library_sync_library` | fail-safe default (no pattern matched) |
| Manager | SyncLibraryItem(item *Item, force bool) | tier2 | vcenter-only | `vmware_library_sync_library_item` | fail-safe default (no pattern matched) |
| Manager | UpdateLibrary(l *Library) | tier2 | vcenter-only | `vmware_library_update_library` | fail-safe default (no pattern matched) |
| Manager | UpdateLibraryItem(item *Item) | tier2 | vcenter-only | `vmware_library_update_library_item` | fail-safe default (no pattern matched) |
| Manager | ValidateLibraryItemUpdateSessionFile(sessionID string) | tier2 | vcenter-only | `vmware_library_validate_library_item_update_session_file` | fail-safe default (no pattern matched) |
| Manager | WaitOnLibraryItemUpdateSession(sessionID string, interval time.Duration, intervalCallback *ast.FuncType) | — | vcenter-only | `vmware_library_wait_on_library_item_update_session` | name matches read-only pattern |

## vapi/library/finder (3 métodos)

| Receiver | Método | Tier | Modo | Tool proposta | Regra |
|---|---|---|---|---|---|
| Finder | Find(ipath ...string) | — | vcenter-only | `vmware_library_finder_find` | name matches read-only pattern |
| PathFinder | Path(r FindResult) | tier2 | vcenter-only | `vmware_library_finder_path` | fail-safe default (no pattern matched) |
| PathFinder | ResolveLibraryItemStorage(datacenter *object.Datacenter, datastoreMap *ast.MapType, storage []library.Storage) | tier2 | vcenter-only | `vmware_library_finder_resolve_library_item_storage` | fail-safe default (no pattern matched) |

## vapi/namespace (44 métodos)

| Receiver | Método | Tier | Modo | Tool proposta | Regra |
|---|---|---|---|---|---|
| Manager | ActivateSupervisorServiceVersion(id string, version string) | tier2 | vcenter-only | `vmware_namespace_activate_supervisor_service_version` | fail-safe default (no pattern matched) |
| Manager | ActivateSupervisorServices(id string) | tier2 | vcenter-only | `vmware_namespace_activate_supervisor_services` | fail-safe default (no pattern matched) |
| Manager | CreateClusterNetwork(clusterID string, spec *NetworkCreateSpec) | tier2 | vcenter-only | `vmware_namespace_create_cluster_network` | fail-safe default (no pattern matched) |
| Manager | CreateNamespace(spec NamespacesInstanceCreateSpec) | tier2 | vcenter-only | `vmware_namespace_create_namespace` | fail-safe default (no pattern matched) |
| Manager | CreateNamespaceV2(spec NamespaceInstanceCreateSpecV2) | tier2 | vcenter-only | `vmware_namespace_create_namespace_v2` | fail-safe default (no pattern matched) |
| Manager | CreateSupervisorService(service *SupervisorService) | tier2 | vcenter-only | `vmware_namespace_create_supervisor_service` | fail-safe default (no pattern matched) |
| Manager | CreateSupervisorServiceVersion(id string, service *SupervisorServiceVersion) | tier2 | vcenter-only | `vmware_namespace_create_supervisor_service_version` | fail-safe default (no pattern matched) |
| Manager | CreateSupportBundle(id string) | tier2 | vcenter-only | `vmware_namespace_create_support_bundle` | fail-safe default (no pattern matched) |
| Manager | CreateVmClass(spec VirtualMachineClassCreateSpec) | tier2 | vcenter-only | `vmware_namespace_create_vm_class` | fail-safe default (no pattern matched) |
| Manager | DeactivateSupervisorServiceVersion(id string, version string) | tier2 | vcenter-only | `vmware_namespace_deactivate_supervisor_service_version` | fail-safe default (no pattern matched) |
| Manager | DeactivateSupervisorServices(id string) | tier2 | vcenter-only | `vmware_namespace_deactivate_supervisor_services` | fail-safe default (no pattern matched) |
| Manager | DeleteClusterNetwork(clusterID string, networkID string) | tier1 | vcenter-only | `vmware_namespace_delete_cluster_network` | name matches tier1 pattern |
| Manager | DeleteNamespace(namespace string) | tier1 | vcenter-only | `vmware_namespace_delete_namespace` | name matches tier1 pattern |
| Manager | DeleteVmClass(vmClass string) | tier1 | vcenter-only | `vmware_namespace_delete_vm_class` | name matches tier1 pattern |
| Manager | DisableCluster(id string) | tier2 | vcenter-only | `vmware_namespace_disable_cluster` | name matches tier2 pattern |
| Manager | EnableCluster(id string, spec *EnableClusterSpec) | tier2 | vcenter-only | `vmware_namespace_enable_cluster` | fail-safe default (no pattern matched) |
| Manager | EnableOnComputeCluster(id string, spec *EnableOnComputeClusterSpec) | tier2 | vcenter-only | `vmware_namespace_enable_on_compute_cluster` | fail-safe default (no pattern matched) |
| Manager | EnableOnZones(spec *EnableOnZonesSpec) | tier2 | vcenter-only | `vmware_namespace_enable_on_zones` | fail-safe default (no pattern matched) |
| Manager | GetClusterNetwork(clusterID string, networkID string) | — | vcenter-only | `vmware_namespace_get_cluster_network` | name matches read-only pattern |
| Manager | GetNamespace(namespace string) | — | vcenter-only | `vmware_namespace_get_namespace` | name matches read-only pattern |
| Manager | GetNamespaceV2(namespace string) | — | vcenter-only | `vmware_namespace_get_namespace_v2` | name matches read-only pattern |
| Manager | GetSupervisorService(id string) | — | vcenter-only | `vmware_namespace_get_supervisor_service` | name matches read-only pattern |
| Manager | GetSupervisorServiceVersion(id string, version string) | — | vcenter-only | `vmware_namespace_get_supervisor_service_version` | name matches read-only pattern |
| Manager | GetSupervisorSummaries() | — | vcenter-only | `vmware_namespace_get_supervisor_summaries` | name matches read-only pattern |
| Manager | GetSupervisorSummary(id string) | — | vcenter-only | `vmware_namespace_get_supervisor_summary` | name matches read-only pattern |
| Manager | GetSupervisorTopology(id string) | — | vcenter-only | `vmware_namespace_get_supervisor_topology` | name matches read-only pattern |
| Manager | GetVmClass(vmClass string) | — | vcenter-only | `vmware_namespace_get_vm_class` | name matches read-only pattern |
| Manager | ListClusterNetworks(clusterID string) | — | vcenter-only | `vmware_namespace_list_cluster_networks` | name matches read-only pattern |
| Manager | ListClusters() | — | vcenter-only | `vmware_namespace_list_clusters` | name matches read-only pattern |
| Manager | ListCompatibleDistributedSwitches(clusterId string) | — | vcenter-only | `vmware_namespace_list_compatible_distributed_switches` | name matches read-only pattern |
| Manager | ListCompatibleEdgeClusters(clusterId string, switchId string) | — | vcenter-only | `vmware_namespace_list_compatible_edge_clusters` | name matches read-only pattern |
| Manager | ListNamespaces() | — | vcenter-only | `vmware_namespace_list_namespaces` | name matches read-only pattern |
| Manager | ListNamespacesV2() | — | vcenter-only | `vmware_namespace_list_namespaces_v2` | name matches read-only pattern |
| Manager | ListSupervisorServiceVersions(id string) | — | vcenter-only | `vmware_namespace_list_supervisor_service_versions` | name matches read-only pattern |
| Manager | ListSupervisorServices() | — | vcenter-only | `vmware_namespace_list_supervisor_services` | name matches read-only pattern |
| Manager | ListVmClasses() | — | vcenter-only | `vmware_namespace_list_vm_classes` | name matches read-only pattern |
| Manager | RegisterVM(namespace string, spec RegisterVMSpec) | tier2 | vcenter-only | `vmware_namespace_register_vm` | fail-safe default (no pattern matched) |
| Manager | RemoveSupervisorService(id string) | tier1 | vcenter-only | `vmware_namespace_remove_supervisor_service` | name matches tier1 pattern |
| Manager | RemoveSupervisorServiceVersion(id string, version string) | tier1 | vcenter-only | `vmware_namespace_remove_supervisor_service_version` | name matches tier1 pattern |
| Manager | SetClusterNetwork(clusterID string, networkID string, spec *NetworkSetSpec) | tier2 | vcenter-only | `vmware_namespace_set_cluster_network` | fail-safe default (no pattern matched) |
| Manager | SupportBundleRequest(bundle *SupportBundleLocation) | tier2 | vcenter-only | `vmware_namespace_support_bundle_request` | fail-safe default (no pattern matched) |
| Manager | UpdateClusterNetwork(clusterID string, networkID string, spec *NetworkUpdateSpec) | tier2 | vcenter-only | `vmware_namespace_update_cluster_network` | fail-safe default (no pattern matched) |
| Manager | UpdateNamespace(namespace string, spec NamespacesInstanceUpdateSpec) | tier2 | vcenter-only | `vmware_namespace_update_namespace` | fail-safe default (no pattern matched) |
| Manager | UpdateVmClass(vmClass string, spec VirtualMachineClassUpdateSpec) | tier2 | vcenter-only | `vmware_namespace_update_vm_class` | fail-safe default (no pattern matched) |

## vapi/rest (11 métodos)

| Receiver | Método | Tier | Modo | Tool proposta | Regra |
|---|---|---|---|---|---|
| Client | Do(req *http.Request, resBody any) | tier2 | vcenter-only | `vmware_rest_do` | fail-safe default (no pattern matched) |
| Client | Download(u *url.URL, param *soap.Download) | tier2 | vcenter-only | `vmware_rest_download` | fail-safe default (no pattern matched) |
| Client | DownloadAttachment(req *http.Request, filename string) | tier2 | vcenter-only | `vmware_rest_download_attachment` | fail-safe default (no pattern matched) |
| Client | DownloadFile(file string, u *url.URL, param *soap.Download) | tier2 | vcenter-only | `vmware_rest_download_file` | fail-safe default (no pattern matched) |
| Client | Login(user *url.Userinfo) | tier2 | vcenter-only | `vmware_rest_login` | fail-safe default (no pattern matched) |
| Client | LoginByToken() | tier2 | vcenter-only | `vmware_rest_login_by_token` | fail-safe default (no pattern matched) |
| Client | Logout() | tier2 | vcenter-only | `vmware_rest_logout` | fail-safe default (no pattern matched) |
| Client | Session() | tier2 | vcenter-only | `vmware_rest_session` | fail-safe default (no pattern matched) |
| Client | Upload(f io.Reader, u *url.URL, param *soap.Upload) | tier2 | vcenter-only | `vmware_rest_upload` | fail-safe default (no pattern matched) |
| Client | WithHeader(headers http.Header) | tier2 | vcenter-only | `vmware_rest_with_header` | fail-safe default (no pattern matched) |
| Client | WithSigner(s Signer) | tier2 | vcenter-only | `vmware_rest_with_signer` | fail-safe default (no pattern matched) |

## vapi/tags (27 métodos)

| Receiver | Método | Tier | Modo | Tool proposta | Regra |
|---|---|---|---|---|---|
| Manager | AttachMultipleTagsToObject(tagIDs []string, ref mo.Reference) | tier2 | vcenter-only | `vmware_tags_attach_multiple_tags_to_object` | fail-safe default (no pattern matched) |
| Manager | AttachTag(tagID string, ref mo.Reference) | tier2 | vcenter-only | `vmware_tags_attach_tag` | fail-safe default (no pattern matched) |
| Manager | AttachTagToMultipleObjects(tagID string, refs []mo.Reference) | tier2 | vcenter-only | `vmware_tags_attach_tag_to_multiple_objects` | fail-safe default (no pattern matched) |
| Manager | CreateCategory(category *Category) | tier2 | vcenter-only | `vmware_tags_create_category` | fail-safe default (no pattern matched) |
| Manager | CreateTag(tag *Tag) | tier2 | vcenter-only | `vmware_tags_create_tag` | fail-safe default (no pattern matched) |
| Manager | DeleteCategory(category *Category) | tier1 | vcenter-only | `vmware_tags_delete_category` | name matches tier1 pattern |
| Manager | DeleteTag(tag *Tag) | tier1 | vcenter-only | `vmware_tags_delete_tag` | name matches tier1 pattern |
| Manager | DetachMultipleTagsFromObject(tagIDs []string, ref mo.Reference) | tier1 | vcenter-only | `vmware_tags_detach_multiple_tags_from_object` | name matches tier1 pattern |
| Manager | DetachTag(tagID string, ref mo.Reference) | tier1 | vcenter-only | `vmware_tags_detach_tag` | name matches tier1 pattern |
| Manager | GetAttachedObjectsOnTags(tagID []string) | — | vcenter-only | `vmware_tags_get_attached_objects_on_tags` | name matches read-only pattern |
| Manager | GetAttachedTags(ref mo.Reference) | — | vcenter-only | `vmware_tags_get_attached_tags` | name matches read-only pattern |
| Manager | GetAttachedTagsOnObjects(objectID []mo.Reference) | — | vcenter-only | `vmware_tags_get_attached_tags_on_objects` | name matches read-only pattern |
| Manager | GetCategories() | — | vcenter-only | `vmware_tags_get_categories` | name matches read-only pattern |
| Manager | GetCategory(id string) | — | vcenter-only | `vmware_tags_get_category` | name matches read-only pattern |
| Manager | GetTag(id string) | — | vcenter-only | `vmware_tags_get_tag` | name matches read-only pattern |
| Manager | GetTagForCategory(id string, category string) | — | vcenter-only | `vmware_tags_get_tag_for_category` | name matches read-only pattern |
| Manager | GetTags() | — | vcenter-only | `vmware_tags_get_tags` | name matches read-only pattern |
| Manager | GetTagsForCategory(id string) | — | vcenter-only | `vmware_tags_get_tags_for_category` | name matches read-only pattern |
| Manager | ListAttachedObjects(tagID string) | — | vcenter-only | `vmware_tags_list_attached_objects` | name matches read-only pattern |
| Manager | ListAttachedObjectsOnTags(tagID []string) | — | vcenter-only | `vmware_tags_list_attached_objects_on_tags` | name matches read-only pattern |
| Manager | ListAttachedTags(ref mo.Reference) | — | vcenter-only | `vmware_tags_list_attached_tags` | name matches read-only pattern |
| Manager | ListAttachedTagsOnObjects(objectID []mo.Reference) | — | vcenter-only | `vmware_tags_list_attached_tags_on_objects` | name matches read-only pattern |
| Manager | ListCategories() | — | vcenter-only | `vmware_tags_list_categories` | name matches read-only pattern |
| Manager | ListTags() | — | vcenter-only | `vmware_tags_list_tags` | name matches read-only pattern |
| Manager | ListTagsForCategory(id string) | — | vcenter-only | `vmware_tags_list_tags_for_category` | name matches read-only pattern |
| Manager | UpdateCategory(category *Category) | tier2 | vcenter-only | `vmware_tags_update_category` | fail-safe default (no pattern matched) |
| Manager | UpdateTag(tag *Tag) | tier2 | vcenter-only | `vmware_tags_update_tag` | fail-safe default (no pattern matched) |

## vapi/vcenter (10 métodos)

| Receiver | Método | Tier | Modo | Tool proposta | Regra |
|---|---|---|---|---|---|
| Manager | CheckIn(libraryItemID string, vm mo.Reference, checkin *CheckIn) | tier2 | vcenter-only | `vmware_vcenter_check_in` | fail-safe default (no pattern matched) |
| Manager | CheckOut(libraryItemID string, checkout *CheckOut) | tier2 | vcenter-only | `vmware_vcenter_check_out` | fail-safe default (no pattern matched) |
| Manager | CreateOVF(ovf OVF) | tier2 | vcenter-only | `vmware_vcenter_create_ovf` | fail-safe default (no pattern matched) |
| Manager | CreateTemplate(vmtx Template) | tier2 | vcenter-only | `vmware_vcenter_create_template` | fail-safe default (no pattern matched) |
| Manager | DeployLibraryItem(libraryItemID string, deploy Deploy) | tier2 | vcenter-only | `vmware_vcenter_deploy_library_item` | fail-safe default (no pattern matched) |
| Manager | DeployTemplateLibraryItem(libraryItemID string, deploy DeployTemplate) | tier2 | vcenter-only | `vmware_vcenter_deploy_template_library_item` | fail-safe default (no pattern matched) |
| Manager | FilterLibraryItem(libraryItemID string, filter FilterRequest) | tier2 | vcenter-only | `vmware_vcenter_filter_library_item` | fail-safe default (no pattern matched) |
| Manager | GetLibraryTemplateInfo(libraryItemID string) | — | vcenter-only | `vmware_vcenter_get_library_template_info` | name matches read-only pattern |
| Manager | SyncTemplateLibrary(l TemplateLibrary, items ...library.Item) | tier2 | vcenter-only | `vmware_vcenter_sync_template_library` | fail-safe default (no pattern matched) |
| Manager | SyncTemplateLibraryItem(item library.Item, deploy *Deploy, spec *Template) | tier2 | vcenter-only | `vmware_vcenter_sync_template_library_item` | fail-safe default (no pattern matched) |

## vapi/vm/dataset (9 métodos)

| Receiver | Método | Tier | Modo | Tool proposta | Regra |
|---|---|---|---|---|---|
| Manager | CreateDataSet(vm string, spec *CreateSpec) | tier2 | vcenter-only | `vmware_vm_dataset_create_data_set` | fail-safe default (no pattern matched) |
| Manager | DeleteDataSet(vm string, dataSet string, force bool) | tier1 | vcenter-only | `vmware_vm_dataset_delete_data_set` | name matches tier1 pattern |
| Manager | DeleteEntry(vm string, dataSet string, key string) | tier1 | vcenter-only | `vmware_vm_dataset_delete_entry` | name matches tier1 pattern |
| Manager | GetDataSet(vm string, dataSet string) | — | vcenter-only | `vmware_vm_dataset_get_data_set` | name matches read-only pattern |
| Manager | GetEntry(vm string, dataSet string, key string) | — | vcenter-only | `vmware_vm_dataset_get_entry` | name matches read-only pattern |
| Manager | ListDataSets(vm string) | — | vcenter-only | `vmware_vm_dataset_list_data_sets` | name matches read-only pattern |
| Manager | ListEntries(vm string, dataSet string) | — | vcenter-only | `vmware_vm_dataset_list_entries` | name matches read-only pattern |
| Manager | SetEntry(vm string, dataSet string, key string, value string) | tier2 | vcenter-only | `vmware_vm_dataset_set_entry` | fail-safe default (no pattern matched) |
| Manager | UpdateDataSet(vm string, dataSet string, spec *UpdateSpec) | tier2 | vcenter-only | `vmware_vm_dataset_update_data_set` | fail-safe default (no pattern matched) |

