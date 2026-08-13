package tools

import (
	"context"
	"testing"

	"github.com/vmware/govmomi/simulator"
)

// appliancetools/vsphereGeneralTools are the exact catalogs documented in
// the plan ("Catálogo das 29 tools já existentes, separadas por tipo") —
// the tests below assert set-equality against these, not "contains".
var vcenterOnlyTools = []string{
	"vmware_appliance_version",
	"vmware_appliance_uptime",
	"vmware_appliance_health",
	"vmware_appliance_health_detail",
	"vmware_vm_provisioning_check_relocate",
	"vmware_vm_compatibility_check_config",
	"vmware_vm_compatibility_check",
	// Fase 5 "network" group, vcenter-only half (6 — the other 1 is
	// vsphere-general, see vsphereGeneralTools below)
	"vmware_dvpg_reconfigure",
	"vmware_dvs_add_portgroup",
	"vmware_dvs_fetch_dvports",
	"vmware_dvs_reconfigure",
	"vmware_dvs_reconfigure_dvport",
	"vmware_dvs_reconfigure_lacp",
	// Fase 6 "compute/cluster" group, vcenter-only half (4 — the other 9 are
	// vsphere-general, see vsphereGeneralTools below)
	"vmware_cluster_add_host",
	"vmware_cluster_configuration",
	"vmware_cluster_move_into",
	"vmware_cluster_place_vm",
	// Fase 7 "custom-fields" group (6)
	"vmware_custom_field_add",
	"vmware_custom_field_field",
	"vmware_custom_field_find_key",
	"vmware_custom_field_remove",
	"vmware_custom_field_rename",
	"vmware_custom_field_set",
	// Fase 7 "extension" group (6)
	"vmware_extension_find",
	"vmware_extension_list",
	"vmware_extension_register",
	"vmware_extension_set_certificate",
	"vmware_extension_unregister",
	"vmware_extension_update",
	// Fase 7 "customization-spec" group (10)
	"vmware_customization_spec_create",
	"vmware_customization_spec_delete",
	"vmware_customization_spec_does_exist",
	"vmware_customization_spec_duplicate",
	"vmware_customization_spec_get",
	"vmware_customization_spec_info",
	"vmware_customization_spec_item_to_xml",
	"vmware_customization_spec_overwrite",
	"vmware_customization_spec_rename",
	"vmware_customization_spec_xml_to_item",
	// Fase 7 "tenant" group (3)
	"vmware_tenant_mark_service_provider_entities",
	"vmware_tenant_retrieve_service_provider_entities",
	"vmware_tenant_unmark_service_provider_entities",
	// Fase 8a "library-core" group (27)
	"vmware_library_copy_item",
	"vmware_library_create_item",
	"vmware_library_create_library",
	"vmware_library_create_subscriber",
	"vmware_library_delete_item",
	"vmware_library_delete_library",
	"vmware_library_delete_subscriber",
	"vmware_library_evict_subscribed_item",
	"vmware_library_evict_subscribed_library",
	"vmware_library_find_items",
	"vmware_library_find_library",
	"vmware_library_force_delete_library",
	"vmware_library_get_item",
	"vmware_library_get_items",
	"vmware_library_get_libraries",
	"vmware_library_get_library_by_id",
	"vmware_library_get_library_by_name",
	"vmware_library_get_subscriber",
	"vmware_library_list_items",
	"vmware_library_list_libraries",
	"vmware_library_list_subscribers",
	"vmware_library_publish_item",
	"vmware_library_publish_library",
	"vmware_library_sync_item",
	"vmware_library_sync_library",
	"vmware_library_update_item",
	"vmware_library_update_library",
	// Fase 8a "library-sessions" group (26)
	"vmware_library_add_library_item_file",
	"vmware_library_add_library_item_file_from_uri",
	"vmware_library_cancel_library_item_download_session",
	"vmware_library_cancel_library_item_update_session",
	"vmware_library_complete_library_item_update_session",
	"vmware_library_create_library_item_download_session",
	"vmware_library_create_library_item_update_session",
	"vmware_library_delete_library_item_download_session",
	"vmware_library_delete_library_item_update_session",
	"vmware_library_fail_library_item_download_session",
	"vmware_library_fail_library_item_update_session",
	"vmware_library_get_library_item_download_session",
	"vmware_library_get_library_item_download_session_file",
	"vmware_library_get_library_item_update_session",
	"vmware_library_get_library_item_update_session_file",
	"vmware_library_keep_alive_library_item_download_session",
	"vmware_library_keep_alive_library_item_update_session",
	"vmware_library_list_library_item_download_session",
	"vmware_library_list_library_item_download_session_file",
	"vmware_library_list_library_item_update_session",
	"vmware_library_list_library_item_update_session_file",
	"vmware_library_prepare_library_item_download_session_file",
	"vmware_library_probe_transfer_endpoint",
	"vmware_library_remove_library_item_update_session_file",
	"vmware_library_validate_library_item_update_session_file",
	"vmware_library_wait_on_library_item_update_session",
	// Fase 8a "library-misc" group (15)
	"vmware_library_add_library_usage",
	"vmware_library_create_trusted_certificate",
	"vmware_library_default_ovf_security_policy",
	"vmware_library_delete_trusted_certificate",
	"vmware_library_finder_find",
	"vmware_library_get_library_item_file",
	"vmware_library_get_library_item_storage",
	"vmware_library_get_library_usage",
	"vmware_library_get_trusted_certificate",
	"vmware_library_list_library_item_files",
	"vmware_library_list_library_item_storage",
	"vmware_library_list_library_usage",
	"vmware_library_list_security_policies",
	"vmware_library_list_trusted_certificates",
	"vmware_library_remove_library_usage",
	// Fase 8a "tags" group (27)
	"vmware_tags_attach_multiple_tags_to_object",
	"vmware_tags_attach_tag",
	"vmware_tags_attach_tag_to_multiple_objects",
	"vmware_tags_create_category",
	"vmware_tags_create_tag",
	"vmware_tags_delete_category",
	"vmware_tags_delete_tag",
	"vmware_tags_detach_multiple_tags_from_object",
	"vmware_tags_detach_tag",
	"vmware_tags_get_attached_objects_on_tags",
	"vmware_tags_get_attached_tags",
	"vmware_tags_get_attached_tags_on_objects",
	"vmware_tags_get_categories",
	"vmware_tags_get_category",
	"vmware_tags_get_tag",
	"vmware_tags_get_tag_for_category",
	"vmware_tags_get_tags",
	"vmware_tags_get_tags_for_category",
	"vmware_tags_list_attached_objects",
	"vmware_tags_list_attached_objects_on_tags",
	"vmware_tags_list_attached_tags",
	"vmware_tags_list_attached_tags_on_objects",
	"vmware_tags_list_categories",
	"vmware_tags_list_tags",
	"vmware_tags_list_tags_for_category",
	"vmware_tags_update_category",
	"vmware_tags_update_tag",
	// Fase 8a "vcenter-template" group (10)
	"vmware_vcenter_check_in",
	"vmware_vcenter_check_out",
	"vmware_vcenter_create_ovf",
	"vmware_vcenter_create_template",
	"vmware_vcenter_deploy_library_item",
	"vmware_vcenter_deploy_template_library_item",
	"vmware_vcenter_filter_library_item",
	"vmware_vcenter_get_library_template_info",
	"vmware_vcenter_sync_template_library",
	"vmware_vcenter_sync_template_library_item",
	// Fase 8a "namespace-core" group (22)
	"vmware_namespace_create_namespace",
	"vmware_namespace_create_support_bundle",
	"vmware_namespace_create_vm_class",
	"vmware_namespace_delete_namespace",
	"vmware_namespace_delete_vm_class",
	"vmware_namespace_disable_cluster",
	"vmware_namespace_enable_cluster",
	"vmware_namespace_enable_on_compute_cluster",
	"vmware_namespace_enable_on_zones",
	"vmware_namespace_get_namespace",
	"vmware_namespace_get_supervisor_summaries",
	"vmware_namespace_get_supervisor_summary",
	"vmware_namespace_get_supervisor_topology",
	"vmware_namespace_get_vm_class",
	"vmware_namespace_list_clusters",
	"vmware_namespace_list_compatible_distributed_switches",
	"vmware_namespace_list_compatible_edge_clusters",
	"vmware_namespace_list_namespaces",
	"vmware_namespace_list_vm_classes",
	"vmware_namespace_register_vm",
	"vmware_namespace_update_namespace",
	"vmware_namespace_update_vm_class",
	// Fase 8a "namespace-services" group (21)
	"vmware_namespace_activate_supervisor_service_version",
	"vmware_namespace_activate_supervisor_services",
	"vmware_namespace_create_cluster_network",
	"vmware_namespace_create_namespace_v2",
	"vmware_namespace_create_supervisor_service",
	"vmware_namespace_create_supervisor_service_version",
	"vmware_namespace_deactivate_supervisor_service_version",
	"vmware_namespace_deactivate_supervisor_services",
	"vmware_namespace_delete_cluster_network",
	"vmware_namespace_get_cluster_network",
	"vmware_namespace_get_namespace_v2",
	"vmware_namespace_get_supervisor_service",
	"vmware_namespace_get_supervisor_service_version",
	"vmware_namespace_list_cluster_networks",
	"vmware_namespace_list_namespaces_v2",
	"vmware_namespace_list_supervisor_service_versions",
	"vmware_namespace_list_supervisor_services",
	"vmware_namespace_remove_supervisor_service",
	"vmware_namespace_remove_supervisor_service_version",
	"vmware_namespace_set_cluster_network",
	"vmware_namespace_update_cluster_network",
	// Fase 8a "esx-settings-cluster-vms" group (16)
	"vmware_esx_settings_clusters_vms_apply",
	"vmware_esx_settings_clusters_vms_apply_wait_for_completion",
	"vmware_esx_settings_clusters_vms_check_compliance",
	"vmware_esx_settings_clusters_vms_delete_solution",
	"vmware_esx_settings_clusters_vms_enable",
	"vmware_esx_settings_clusters_vms_enable_async",
	"vmware_esx_settings_clusters_vms_get_solution",
	"vmware_esx_settings_clusters_vms_list_hooks",
	"vmware_esx_settings_clusters_vms_list_solutions",
	"vmware_esx_settings_clusters_vms_mark_as_processed",
	"vmware_esx_settings_clusters_vms_multi_source_enable",
	"vmware_esx_settings_clusters_vms_multi_source_enable_async",
	"vmware_esx_settings_clusters_vms_process_dynamic_update",
	"vmware_esx_settings_clusters_vms_set_solution",
	"vmware_esx_settings_clusters_vms_transition",
	"vmware_esx_settings_clusters_vms_transition_async",
	// Fase 8a "cluster-modules" group (6)
	"vmware_cluster_add_module_members",
	"vmware_cluster_create_module",
	"vmware_cluster_delete_module",
	"vmware_cluster_list_module_members",
	"vmware_cluster_list_modules",
	"vmware_cluster_remove_module_members",
	// Fase 8a "crypto" group (3)
	"vmware_crypto_kms_provider_create",
	"vmware_crypto_kms_provider_delete",
	"vmware_crypto_kms_provider_export",
	// Fase 8a "cis-tasks" group (4)
	"vmware_cis_tasks_get",
	"vmware_cis_tasks_wait_for_completion",
	"vmware_cis_tasks_wait_for_running_or_error",
	"vmware_cis_tasks_wait_for_running_or_terminal_state",
	// Fase 8a "vm-dataset" group (9)
	"vmware_vm_dataset_create_data_set",
	"vmware_vm_dataset_delete_data_set",
	"vmware_vm_dataset_delete_entry",
	"vmware_vm_dataset_get_data_set",
	"vmware_vm_dataset_get_entry",
	"vmware_vm_dataset_list_data_sets",
	"vmware_vm_dataset_list_entries",
	"vmware_vm_dataset_set_entry",
	"vmware_vm_dataset_update_data_set",
	// Fase 8a "appliance-small" group (15)
	"vmware_appliance_access_consolecli_get",
	"vmware_appliance_access_consolecli_set",
	"vmware_appliance_access_dcui_get",
	"vmware_appliance_access_dcui_set",
	"vmware_appliance_access_shell_get",
	"vmware_appliance_access_shell_set",
	"vmware_appliance_access_ssh_get",
	"vmware_appliance_access_ssh_set",
	"vmware_appliance_logging_forwarding",
	"vmware_appliance_networking_no_proxy",
	"vmware_appliance_networking_proxy_list",
	"vmware_appliance_shutdown_cancel",
	"vmware_appliance_shutdown_get",
	"vmware_appliance_shutdown_power_off",
	"vmware_appliance_shutdown_reboot",
	// Fase 8a "authentication" group (1)
	"vmware_authentication_issue",
	// Fase 8b "vami-recovery-update" group (31)
	"vmware_appliance_recovery_backup_job_cancel",
	"vmware_appliance_recovery_backup_job_create",
	"vmware_appliance_recovery_backup_job_details",
	"vmware_appliance_recovery_backup_job_list",
	"vmware_appliance_recovery_backup_job_status",
	"vmware_appliance_recovery_backup_part_size",
	"vmware_appliance_recovery_backup_parts",
	"vmware_appliance_recovery_backup_schedule_create",
	"vmware_appliance_recovery_backup_schedule_delete",
	"vmware_appliance_recovery_backup_schedule_get",
	"vmware_appliance_recovery_backup_schedule_list",
	"vmware_appliance_recovery_backup_schedule_run",
	"vmware_appliance_recovery_backup_schedule_update",
	"vmware_appliance_recovery_backup_validate",
	"vmware_appliance_recovery_restore_job_cancel",
	"vmware_appliance_recovery_restore_job_create",
	"vmware_appliance_recovery_restore_job_status",
	"vmware_appliance_update_check_cdrom",
	"vmware_appliance_update_check_last",
	"vmware_appliance_update_check_url_cdrom",
	"vmware_appliance_update_install",
	"vmware_appliance_update_pending_details",
	"vmware_appliance_update_policy_get",
	"vmware_appliance_update_policy_set",
	"vmware_appliance_update_precheck",
	"vmware_appliance_update_stage",
	"vmware_appliance_update_stage_and_install",
	"vmware_appliance_update_staged_delete",
	"vmware_appliance_update_staged_get",
	"vmware_appliance_update_status",
	"vmware_appliance_update_validate",
	// Fase 8b "vami-network-system" group (19)
	"vmware_appliance_health_lastcheck",
	"vmware_appliance_monitoring_item",
	"vmware_appliance_monitoring_list",
	"vmware_appliance_monitoring_query",
	"vmware_appliance_network_dns_domains_add",
	"vmware_appliance_network_dns_domains_list",
	"vmware_appliance_network_dns_domains_set",
	"vmware_appliance_network_dns_hostname",
	"vmware_appliance_network_dns_hostname_set",
	"vmware_appliance_network_dns_hostname_test",
	"vmware_appliance_network_dns_servers_add",
	"vmware_appliance_network_dns_servers_list",
	"vmware_appliance_network_dns_servers_set",
	"vmware_appliance_network_dns_servers_test",
	"vmware_appliance_network_interface_details",
	"vmware_appliance_network_interfaces_list",
	"vmware_appliance_system_storage",
	"vmware_appliance_system_storage_resize",
	"vmware_appliance_system_time",
	// Fase 8b "vami-techpreview-network" group (27)
	"vmware_appliance_techpreview_firewall_create",
	"vmware_appliance_techpreview_firewall_delete",
	"vmware_appliance_techpreview_firewall_list",
	"vmware_appliance_techpreview_firewall_replace",
	"vmware_appliance_techpreview_ipv4_details",
	"vmware_appliance_techpreview_ipv4_get",
	"vmware_appliance_techpreview_ipv4_renew",
	"vmware_appliance_techpreview_ipv4_set",
	"vmware_appliance_techpreview_ipv6_details",
	"vmware_appliance_techpreview_ipv6_get",
	"vmware_appliance_techpreview_ipv6_set",
	"vmware_appliance_techpreview_ntp_get",
	"vmware_appliance_techpreview_ntp_server_add",
	"vmware_appliance_techpreview_ntp_server_delete",
	"vmware_appliance_techpreview_ntp_server_set",
	"vmware_appliance_techpreview_ntp_test",
	"vmware_appliance_techpreview_proxy_delete",
	"vmware_appliance_techpreview_proxy_get",
	"vmware_appliance_techpreview_proxy_set",
	"vmware_appliance_techpreview_proxy_test",
	"vmware_appliance_techpreview_routes_add",
	"vmware_appliance_techpreview_routes_delete",
	"vmware_appliance_techpreview_routes_list",
	"vmware_appliance_techpreview_routes_set",
	"vmware_appliance_techpreview_routes_test",
	"vmware_appliance_techpreview_timesync_get",
	"vmware_appliance_techpreview_timesync_set",
	// Fase 8b "vami-services-accounts-vmon" group (27)
	"vmware_appliance_techpreview_local_accounts_create",
	"vmware_appliance_techpreview_local_accounts_delete",
	"vmware_appliance_techpreview_local_accounts_get",
	"vmware_appliance_techpreview_local_accounts_list",
	"vmware_appliance_techpreview_local_accounts_update",
	"vmware_appliance_techpreview_services_control",
	"vmware_appliance_techpreview_services_get",
	"vmware_appliance_techpreview_services_list",
	"vmware_appliance_techpreview_services_restart",
	"vmware_appliance_techpreview_services_stop",
	"vmware_appliance_techpreview_snmp_disable",
	"vmware_appliance_techpreview_snmp_enable",
	"vmware_appliance_techpreview_snmp_generate_hash",
	"vmware_appliance_techpreview_snmp_get",
	"vmware_appliance_techpreview_snmp_limits",
	"vmware_appliance_techpreview_snmp_reset",
	"vmware_appliance_techpreview_snmp_set",
	"vmware_appliance_techpreview_snmp_stats",
	"vmware_appliance_techpreview_snmp_test",
	"vmware_appliance_techpreview_system_update_get",
	"vmware_appliance_techpreview_system_update_set",
	"vmware_appliance_vmon_service_get",
	"vmware_appliance_vmon_service_list",
	"vmware_appliance_vmon_service_restart",
	"vmware_appliance_vmon_service_start",
	"vmware_appliance_vmon_service_stop",
	"vmware_appliance_vmon_service_update",
	// Fase 8b "vami-access-shutdown" group (12)
	"vmware_appliance_access_legacy_consolecli_get",
	"vmware_appliance_access_legacy_consolecli_set",
	"vmware_appliance_access_legacy_dcui_get",
	"vmware_appliance_access_legacy_dcui_set",
	"vmware_appliance_access_legacy_shell_get",
	"vmware_appliance_access_legacy_shell_set",
	"vmware_appliance_access_legacy_ssh_get",
	"vmware_appliance_access_legacy_ssh_set",
	"vmware_appliance_techpreview_shutdown_cancel",
	"vmware_appliance_techpreview_shutdown_get",
	"vmware_appliance_techpreview_shutdown_poweroff",
	"vmware_appliance_techpreview_shutdown_restart",
}

// workstationTools is the Fase 9 catalog — a separate class from
// vcenterOnlyTools/vsphereGeneralTools (modeWorkstation, not
// modeVCenterOnly/modeVSphereGeneral), only reachable via --workstation-url
// today (see TestMode_All's doc comment).
var workstationTools = []string{
	// "workstation-vm" group (11)
	"vmware_workstation_vm_clone",
	"vmware_workstation_vm_config_param_get",
	"vmware_workstation_vm_config_param_set",
	"vmware_workstation_vm_delete",
	"vmware_workstation_vm_get",
	"vmware_workstation_vm_list",
	"vmware_workstation_vm_power_get",
	"vmware_workstation_vm_power_set",
	"vmware_workstation_vm_register",
	"vmware_workstation_vm_restrictions",
	"vmware_workstation_vm_update",
	// "workstation-network" group (17)
	"vmware_workstation_nic_create",
	"vmware_workstation_nic_delete",
	"vmware_workstation_nic_ip",
	"vmware_workstation_nic_list",
	"vmware_workstation_nic_update",
	"vmware_workstation_shared_folder_create",
	"vmware_workstation_shared_folder_delete",
	"vmware_workstation_shared_folder_list",
	"vmware_workstation_shared_folder_update",
	"vmware_workstation_vm_nic_ips",
	"vmware_workstation_vmnet_create",
	"vmware_workstation_vmnet_list",
	"vmware_workstation_vmnet_mactoip_list",
	"vmware_workstation_vmnet_mactoip_set",
	"vmware_workstation_vmnet_portforward_delete",
	"vmware_workstation_vmnet_portforward_list",
	"vmware_workstation_vmnet_portforward_set",
}

// cloudAWSTools is the Fase 10 catalog (VMware Cloud on AWS) — the final
// domain of the plan's "100%" target, only reachable via --cloud-aws-url.
var cloudAWSTools = []string{
	// "cloudaws-orgs" group (29)
	"vmware_cloudaws_account_link_compatible_subnets_async_create",
	"vmware_cloudaws_account_link_compatible_subnets_async_get",
	"vmware_cloudaws_account_link_compatible_subnets_calculate",
	"vmware_cloudaws_account_link_connected_accounts_list",
	"vmware_cloudaws_account_link_delete",
	"vmware_cloudaws_account_link_map_customer_zones",
	"vmware_cloudaws_account_link_sddc_connections_list",
	"vmware_cloudaws_account_link_url_create",
	"vmware_cloudaws_org_details",
	"vmware_cloudaws_org_list",
	"vmware_cloudaws_provider_list",
	"vmware_cloudaws_reservation_list",
	"vmware_cloudaws_reservation_maintenance_window_get",
	"vmware_cloudaws_reservation_maintenance_window_update",
	"vmware_cloudaws_sddc_template_delete",
	"vmware_cloudaws_sddc_template_details",
	"vmware_cloudaws_sddc_template_for_sddc",
	"vmware_cloudaws_sddc_template_list",
	"vmware_cloudaws_storage_cluster_constraints_list",
	"vmware_cloudaws_subscription_create",
	"vmware_cloudaws_subscription_details",
	"vmware_cloudaws_subscription_offers_list",
	"vmware_cloudaws_subscription_products_list",
	"vmware_cloudaws_support_window_list",
	"vmware_cloudaws_support_window_update",
	"vmware_cloudaws_task_action",
	"vmware_cloudaws_task_details",
	"vmware_cloudaws_task_list",
	"vmware_cloudaws_task_list_filtered",
	// "cloudaws-sddcs" group (23)
	"vmware_cloudaws_sddc_addon_credential_create",
	"vmware_cloudaws_sddc_addon_credential_get",
	"vmware_cloudaws_sddc_addon_credential_list",
	"vmware_cloudaws_sddc_addon_credential_update",
	"vmware_cloudaws_sddc_cluster_create",
	"vmware_cloudaws_sddc_cluster_delete",
	"vmware_cloudaws_sddc_convert",
	"vmware_cloudaws_sddc_create",
	"vmware_cloudaws_sddc_delete",
	"vmware_cloudaws_sddc_dns_update_private",
	"vmware_cloudaws_sddc_dns_update_public",
	"vmware_cloudaws_sddc_edrs_cluster_list",
	"vmware_cloudaws_sddc_edrs_cluster_set",
	"vmware_cloudaws_sddc_edrs_list",
	"vmware_cloudaws_sddc_get",
	"vmware_cloudaws_sddc_hosts_update",
	"vmware_cloudaws_sddc_list",
	"vmware_cloudaws_sddc_publicip_create",
	"vmware_cloudaws_sddc_publicip_delete",
	"vmware_cloudaws_sddc_publicip_get",
	"vmware_cloudaws_sddc_publicip_list",
	"vmware_cloudaws_sddc_publicip_update",
	"vmware_cloudaws_sddc_update",
	// "cloudaws-networking-core" group (19)
	"vmware_cloudaws_network_create",
	"vmware_cloudaws_network_delete",
	"vmware_cloudaws_network_firewall_delete",
	"vmware_cloudaws_network_firewall_list",
	"vmware_cloudaws_network_firewall_rule_create",
	"vmware_cloudaws_network_firewall_rule_delete",
	"vmware_cloudaws_network_firewall_rule_get",
	"vmware_cloudaws_network_firewall_rule_stats",
	"vmware_cloudaws_network_firewall_rule_update",
	"vmware_cloudaws_network_firewall_update",
	"vmware_cloudaws_network_get",
	"vmware_cloudaws_network_list",
	"vmware_cloudaws_network_nat_delete",
	"vmware_cloudaws_network_nat_list",
	"vmware_cloudaws_network_nat_rule_create",
	"vmware_cloudaws_network_nat_rule_delete",
	"vmware_cloudaws_network_nat_rule_update",
	"vmware_cloudaws_network_nat_update",
	"vmware_cloudaws_network_update",
	// "cloudaws-networking-edge" group (24)
	"vmware_cloudaws_network_connectivity_test_list",
	"vmware_cloudaws_network_connectivity_test_run",
	"vmware_cloudaws_network_edge_dhcp_leases_list",
	"vmware_cloudaws_network_edge_dns_config_delete",
	"vmware_cloudaws_network_edge_dns_config_get",
	"vmware_cloudaws_network_edge_dns_config_update",
	"vmware_cloudaws_network_edge_dns_statistics_get",
	"vmware_cloudaws_network_edge_dns_status_set",
	"vmware_cloudaws_network_edge_list",
	"vmware_cloudaws_network_edge_peerconfig_list",
	"vmware_cloudaws_network_edge_stats_dashboard_firewall",
	"vmware_cloudaws_network_edge_stats_dashboard_interface",
	"vmware_cloudaws_network_edge_stats_dashboard_ipsec",
	"vmware_cloudaws_network_edge_stats_interfaces",
	"vmware_cloudaws_network_edge_stats_interfaces_uplink",
	"vmware_cloudaws_network_edge_status_get",
	"vmware_cloudaws_network_edge_vnics_list",
	"vmware_cloudaws_network_ipsec_config_delete",
	"vmware_cloudaws_network_ipsec_config_get",
	"vmware_cloudaws_network_ipsec_config_update",
	"vmware_cloudaws_network_ipsec_statistics_get",
	"vmware_cloudaws_network_l2vpn_config_delete",
	"vmware_cloudaws_network_l2vpn_config_update",
	"vmware_cloudaws_network_l2vpn_statistics_get",
}

var vsphereGeneralTools = []string{
	"vmware_about",
	"vmware_list_vms",
	"vmware_list_hosts",
	"vmware_list_datastores",
	"vmware_list_networks",
	"vmware_list_resource_pools",
	"vmware_list_clusters",
	"vmware_list_datacenters",
	"vmware_vm_power_on",
	"vmware_vm_power_off",
	"vmware_vm_reset",
	"vmware_vm_suspend",
	"vmware_vm_reconfigure",
	"vmware_vm_destroy",
	"vmware_vm_snapshot_create",
	"vmware_vm_snapshot_revert",
	"vmware_vm_snapshot_remove",
	"vmware_vm_snapshot_list",
	"vmware_vm_info",
	"vmware_host_maintenance_enter",
	"vmware_host_maintenance_exit",
	"vmware_host_reconnect",
	"vmware_host_management_ips",
	"vmware_host_info",
	"vmware_datastore_upload_file",
	"vmware_host_option_query",
	"vmware_host_option_update",
	"vmware_vm_snapshot_find",
	"vmware_vm_snapshot_create_ex",
	"vmware_vm_snapshot_revert_current",
	"vmware_vm_snapshot_remove_all",
	// Fase 2 "lifecycle" group (25)
	"vmware_vm_wait_for_power_state",
	"vmware_vm_wait_for_net_ip",
	"vmware_vm_wait_for_ip",
	"vmware_vm_is_tools_running",
	"vmware_vm_is_template",
	"vmware_vm_query_changed_disk_areas",
	"vmware_vm_boot_options",
	"vmware_vm_device",
	"vmware_vm_host_system",
	"vmware_vm_resource_pool",
	"vmware_vm_standby_guest",
	"vmware_vm_shutdown_guest",
	"vmware_vm_reboot_guest",
	"vmware_vm_mark_as_template",
	"vmware_vm_mark_as_virtual_machine",
	"vmware_vm_set_boot_options",
	"vmware_vm_refresh_storage_info",
	"vmware_vm_upgrade_vm",
	"vmware_vm_upgrade_tools",
	"vmware_vm_mount_tools_installer",
	"vmware_vm_unmount_tools_installer",
	"vmware_vm_answer",
	"vmware_vm_acquire_ticket",
	"vmware_vm_put_usb_scan_codes",
	"vmware_vm_unregister",
	// Fase 2 "device" group (5)
	"vmware_vm_add_device",
	"vmware_vm_edit_device",
	"vmware_vm_attach_disk",
	"vmware_vm_remove_device",
	"vmware_vm_detach_disk",
	// Fase 2 "provisioning" group, vsphere-general half (8 — the other 3 are
	// vcenter-only, see vcenterOnlyTools above)
	"vmware_vm_clone",
	"vmware_vm_instant_clone",
	"vmware_vm_relocate",
	"vmware_vm_migrate",
	"vmware_vm_customize",
	"vmware_vm_export",
	"vmware_vm_export_snapshot",
	"vmware_vm_promote_disks",
	// Fase 3 "storage" group (20)
	"vmware_host_datastore_create_local",
	"vmware_host_datastore_create_nas",
	"vmware_host_datastore_create_vmfs",
	"vmware_host_datastore_query_available_disks_for_vmfs",
	"vmware_host_datastore_query_vmfs_datastore_create_options",
	"vmware_host_datastore_remove",
	"vmware_host_datastore_resignature_unresolved_vmfs_volumes",
	"vmware_host_storage_attach_scsi_lun",
	"vmware_host_storage_compute_disk_partition_info",
	"vmware_host_storage_mark_as_local",
	"vmware_host_storage_mark_as_non_local",
	"vmware_host_storage_mark_as_non_ssd",
	"vmware_host_storage_mark_as_ssd",
	"vmware_host_storage_query_unresolved_vmfs_volumes",
	"vmware_host_storage_refresh",
	"vmware_host_storage_rescan_all_hba",
	"vmware_host_storage_rescan_vmfs",
	"vmware_host_storage_retrieve_disk_partition_info",
	"vmware_host_storage_unmount_vmfs_volume",
	"vmware_host_storage_update_disk_partition_info",
	// Fase 3 "network" group (21)
	"vmware_host_network_add_port_group",
	"vmware_host_network_add_service_console_virtual_nic",
	"vmware_host_network_add_virtual_nic",
	"vmware_host_network_add_virtual_switch",
	"vmware_host_network_query_network_hint",
	"vmware_host_network_refresh",
	"vmware_host_network_remove_port_group",
	"vmware_host_network_remove_service_console_virtual_nic",
	"vmware_host_network_remove_virtual_nic",
	"vmware_host_network_remove_virtual_switch",
	"vmware_host_network_restart_service_console_virtual_nic",
	"vmware_host_network_update_console_ip_route_config",
	"vmware_host_network_update_dns_config",
	"vmware_host_network_update_ip_route_config",
	"vmware_host_network_update_ip_route_table_config",
	"vmware_host_network_update_network_config",
	"vmware_host_network_update_physical_nic_link_speed",
	"vmware_host_network_update_port_group",
	"vmware_host_network_update_service_console_virtual_nic",
	"vmware_host_network_update_virtual_nic",
	"vmware_host_network_update_virtual_switch",
	// Fase 3 "security" group (14)
	"vmware_host_account_create",
	"vmware_host_account_remove",
	"vmware_host_account_update",
	"vmware_host_certificate_generate_csr",
	"vmware_host_certificate_generate_csr_by_dn",
	"vmware_host_certificate_info",
	"vmware_host_certificate_install_server_certificate",
	"vmware_host_certificate_list_ca_certificates",
	"vmware_host_certificate_list_ca_crls",
	"vmware_host_certificate_replace_ca_certs_and_crls",
	"vmware_host_firewall_disable_ruleset",
	"vmware_host_firewall_enable_ruleset",
	"vmware_host_firewall_info",
	"vmware_host_firewall_refresh",
	// Fase 3 "misc" group (15)
	"vmware_host_datetime_query",
	"vmware_host_datetime_update",
	"vmware_host_datetime_update_config",
	"vmware_host_nic_deselect_vnic",
	"vmware_host_nic_info",
	"vmware_host_nic_select_vnic",
	"vmware_host_service_list",
	"vmware_host_service_restart",
	"vmware_host_service_start",
	"vmware_host_service_stop",
	"vmware_host_service_update_policy",
	"vmware_host_vsan_internal_delete_objects",
	"vmware_host_vsan_internal_get_obj_ext_attrs",
	"vmware_host_vsan_internal_query_object_uuids",
	"vmware_host_vsan_update",
	// Fase 4 "datastore browser/namespace" group (12)
	"vmware_datastore_attached_cluster_hosts",
	"vmware_datastore_attached_hosts",
	"vmware_datastore_download_file",
	"vmware_datastore_namespace_convert_path_to_uuid",
	"vmware_datastore_namespace_create_directory",
	"vmware_datastore_namespace_delete_directory",
	"vmware_datastore_open",
	"vmware_datastore_search",
	"vmware_datastore_search_subfolders",
	"vmware_datastore_service_ticket",
	"vmware_datastore_stat",
	"vmware_datastore_type",
	// Fase 4 "file managers" group (11)
	"vmware_datastore_file_copy",
	"vmware_datastore_file_copy_file",
	"vmware_datastore_file_delete",
	"vmware_datastore_file_delete_file",
	"vmware_datastore_file_delete_virtual_disk",
	"vmware_datastore_file_move",
	"vmware_datastore_file_move_file",
	"vmware_file_copy_datastore_file",
	"vmware_file_delete_datastore_file",
	"vmware_file_make_directory",
	"vmware_file_move_datastore_file",
	// Fase 4 "storage DRS" group (9)
	"vmware_storage_apply_drs_recommendation",
	"vmware_storage_apply_drs_recommendation_to_pod",
	"vmware_storage_cancel_drs_recommendation",
	"vmware_storage_configure_datastore_iorm",
	"vmware_storage_configure_drs_for_pod",
	"vmware_storage_query_datastore_performance_summary",
	"vmware_storage_query_iorm_config_option",
	"vmware_storage_recommend_datastores",
	"vmware_storage_refresh_drs_recommendation",
	// Fase 4 "virtual disk manager" group (11)
	"vmware_virtual_disk_copy",
	"vmware_virtual_disk_create",
	"vmware_virtual_disk_create_child",
	"vmware_virtual_disk_delete_virtual_disk",
	"vmware_virtual_disk_extend",
	"vmware_virtual_disk_inflate",
	"vmware_virtual_disk_move",
	"vmware_virtual_disk_query_info",
	"vmware_virtual_disk_query_uuid",
	"vmware_virtual_disk_set_uuid",
	"vmware_virtual_disk_shrink",
	// Fase 5 "network" group, vsphere-general half (1 — the other 6 are
	// vcenter-only, see vcenterOnlyTools above)
	"vmware_opaque_network_summary",
	// Fase 6 "folder/datacenter" group (14)
	"vmware_datacenter_destroy",
	"vmware_datacenter_folders",
	"vmware_datacenter_power_on_vm",
	"vmware_folder_add_standalone_host",
	"vmware_folder_children",
	"vmware_folder_create_cluster",
	"vmware_folder_create_datacenter",
	"vmware_folder_create_dvs",
	"vmware_folder_create_folder",
	"vmware_folder_create_storage_pod",
	"vmware_folder_create_vm",
	"vmware_folder_move_into",
	"vmware_folder_place_vms_xcluster",
	"vmware_folder_register_vm",
	// Fase 6 "compute/cluster" group, vsphere-general half (9 — the other 4
	// are vcenter-only, see vcenterOnlyTools above)
	"vmware_compute_resource_datastores",
	"vmware_compute_resource_environment_browser",
	"vmware_compute_resource_hosts",
	"vmware_compute_resource_reconfigure",
	"vmware_compute_resource_resource_pool",
	"vmware_environment_query_config_option",
	"vmware_environment_query_config_option_descriptor",
	"vmware_environment_query_config_target",
	"vmware_environment_query_target_capabilities",
	// Fase 6 "resourcepool/vapp" group (13)
	"vmware_resource_pool_create",
	"vmware_resource_pool_create_vapp",
	"vmware_resource_pool_destroy",
	"vmware_resource_pool_destroy_children",
	"vmware_resource_pool_import_vapp",
	"vmware_resource_pool_owner",
	"vmware_resource_pool_update_config",
	"vmware_vapp_clone",
	"vmware_vapp_create_child_vm",
	"vmware_vapp_power_off",
	"vmware_vapp_power_on",
	"vmware_vapp_suspend",
	"vmware_vapp_update_config",
	// Fase 7 "task" group (5)
	"vmware_task_cancel",
	"vmware_task_set_description",
	"vmware_task_set_state",
	"vmware_task_update_progress",
	"vmware_task_wait",
	// Fase 7 "diagnostic" group (4)
	"vmware_diagnostic_browse_log",
	"vmware_diagnostic_generate_log_bundles",
	"vmware_diagnostic_log_copy",
	"vmware_diagnostic_query_descriptions",
	// Fase 7 "search-index" group (9)
	"vmware_search_index_find_all_by_dns_name",
	"vmware_search_index_find_all_by_ip",
	"vmware_search_index_find_all_by_uuid",
	"vmware_search_index_find_by_datastore_path",
	"vmware_search_index_find_by_dns_name",
	"vmware_search_index_find_by_inventory_path",
	"vmware_search_index_find_by_ip",
	"vmware_search_index_find_by_uuid",
	"vmware_search_index_find_child",
	// Fase 7 "authorization" group (14)
	"vmware_authorization_add_role",
	"vmware_authorization_disable_methods",
	"vmware_authorization_enable_methods",
	"vmware_authorization_fetch_user_privilege_on_entities",
	"vmware_authorization_has_privilege_on_entity",
	"vmware_authorization_has_user_privilege_on_entities",
	"vmware_authorization_remove_entity_permission",
	"vmware_authorization_remove_role",
	"vmware_authorization_retrieve_all_permissions",
	"vmware_authorization_retrieve_entity_permissions",
	"vmware_authorization_retrieve_role_permissions",
	"vmware_authorization_role_list",
	"vmware_authorization_set_entity_permissions",
	"vmware_authorization_update_role",
}

func toolNameSet(t *testing.T, r *Registry) map[string]bool {
	t.Helper()
	set := make(map[string]bool)
	for _, tool := range r.ListTools() {
		set[tool.Name] = true
	}
	return set
}

func assertExactToolSet(t *testing.T, r *Registry, expected []string) {
	t.Helper()
	got := toolNameSet(t, r)
	want := make(map[string]bool, len(expected))
	for _, n := range expected {
		want[n] = true
	}
	for n := range want {
		if !got[n] {
			t.Errorf("expected tool %q to be registered, it was not", n)
		}
	}
	for n := range got {
		if !want[n] {
			t.Errorf("tool %q registered but not in the expected set for this mode", n)
		}
	}
	if len(got) != len(want) {
		t.Errorf("expected exactly %d tools, got %d", len(want), len(got))
	}
}

func newModeTestRegistry(t *testing.T, mode ConnectionMode) *Registry {
	t.Helper()
	c, cleanup := newSimClient(t, simulator.ESX())
	t.Cleanup(cleanup)
	return NewRegistry(context.Background(), c, RegistryOptions{ConnectionMode: mode})
}

// TestMode_Unrestricted proves the zero-value ConnectionMode ("", used by
// every other test in this package via RegistryOptions{}) registers every
// tool — this is the backward-compatibility guarantee the retrofit relies
// on: none of the other test files had to change when mode filtering was
// added.
func TestMode_Unrestricted(t *testing.T) {
	r := newModeTestRegistry(t, "")
	all := append(append(append(append([]string{}, vcenterOnlyTools...), vsphereGeneralTools...), workstationTools...), cloudAWSTools...)
	assertExactToolSet(t, r, all)
}

// TestMode_VCenter proves --vcenter-url exposes vcenter-only + general —
// today that's all 29 existing tools (appliance IS vcenter-only, the rest
// is general), same set as unrestricted mode.
func TestMode_VCenter(t *testing.T) {
	r := newModeTestRegistry(t, ConnectionModeVCenter)
	all := append(append([]string{}, vcenterOnlyTools...), vsphereGeneralTools...)
	assertExactToolSet(t, r, all)
}

// TestMode_VMware proves --vmware-url (standalone ESXi) excludes the 4
// appliance/VAMI tools — the core requirement behind this whole feature.
func TestMode_VMware(t *testing.T) {
	r := newModeTestRegistry(t, ConnectionModeVMware)
	assertExactToolSet(t, r, vsphereGeneralTools)

	// Directly confirm none of the vcenter-only tools leaked through, by
	// name, not just by count — a count match alone could hide a swap.
	got := toolNameSet(t, r)
	for _, n := range vcenterOnlyTools {
		if got[n] {
			t.Errorf("vcenter-only tool %q must not appear under --vmware-url (ESXi standalone)", n)
		}
	}
}

// TestMode_Workstation proves --workstation-url registers exactly the 28
// Fase 9 Workstation Pro tools (workstationTools) and none of the vSphere
// tools — the same "no leakage" guarantee TestMode_VMware proves for ESXi.
func TestMode_Workstation(t *testing.T) {
	r := newModeTestRegistry(t, ConnectionModeWorkstation)
	assertExactToolSet(t, r, workstationTools)

	got := toolNameSet(t, r)
	for _, n := range append(append([]string{}, vcenterOnlyTools...), vsphereGeneralTools...) {
		if got[n] {
			t.Errorf("vSphere tool %q must not appear under --workstation-url", n)
		}
	}
}

// TestMode_All proves --vmware-all-url includes vcenter-only + general —
// deliberately still NOT Workstation tools (see main.go's --vmware-all-url
// flag doc comment: joining Workstation into "all" mode needs a 2nd live
// client, an open architecture question from the Fase 0 plan note, not
// resolved by Fase 9 — --workstation-url is the only way to reach the 28
// Workstation tools today).
func TestMode_All(t *testing.T) {
	r := newModeTestRegistry(t, ConnectionModeAll)
	all := append(append([]string{}, vcenterOnlyTools...), vsphereGeneralTools...)
	assertExactToolSet(t, r, all)
}

// TestMode_CloudAWS proves --cloud-aws-url registers exactly the 95
// Fase 10 VMware Cloud on AWS tools (cloudAWSTools) and none of the
// vSphere/Workstation tools — cloud-aws is always isolated, including
// from --vmware-all-url.
func TestMode_CloudAWS(t *testing.T) {
	r := newModeTestRegistry(t, ConnectionModeCloudAWS)
	assertExactToolSet(t, r, cloudAWSTools)

	got := toolNameSet(t, r)
	other := append(append(append([]string{}, vcenterOnlyTools...), vsphereGeneralTools...), workstationTools...)
	for _, n := range other {
		if got[n] {
			t.Errorf("non-CloudAWS tool %q must not appear under --cloud-aws-url", n)
		}
	}
}
