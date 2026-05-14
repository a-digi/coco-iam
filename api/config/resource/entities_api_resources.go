package resource

import (
	"net/http"
	"sync"

	"github.com/a-digi/coco-server/server/request"

	lift_api "github.com/a-digi/coco-lift/resource"
	event "github.com/a-digi/coco-lift/resource/event"
	acl_entity "github.com/a-digi/coco-iam/src/admin/acl/entity"
	admin_groups_entity "github.com/a-digi/coco-iam/src/admin/groups/entity"
	application_admin "github.com/a-digi/coco-iam/src/applications/admin"
	application_cleanup_producers "github.com/a-digi/coco-iam/src/applications/cleanup/producers"
	applications_entity "github.com/a-digi/coco-iam/src/applications/entity"
	organizations_entity "github.com/a-digi/coco-iam/src/organizations/entity"
	profile_listener "github.com/a-digi/coco-iam/src/organizations/profile/listener"
	org_user_listener "github.com/a-digi/coco-iam/src/organizations/users/listener"
	apicred_listener "github.com/a-digi/coco-iam/src/applications/apicredentials/listener"
	oauth_listener "github.com/a-digi/coco-iam/src/oauthserver/listener"
	organization_deleted "github.com/a-digi/coco-iam/src/organizations/deleted"
	organization_groups_admin "github.com/a-digi/coco-iam/src/organizations/groups/admin"
	organization_groups_entity "github.com/a-digi/coco-iam/src/organizations/groups/entity"
	organization_users_admin "github.com/a-digi/coco-iam/src/organizations/users/admin"
	organization_users_entity "github.com/a-digi/coco-iam/src/organizations/users/entity"
	admin_acl "github.com/a-digi/coco-iam/src/admin/acl"
	admin_users "github.com/a-digi/coco-iam/src/admin/users"
	"github.com/a-digi/coco-iam/src/admin/users/entity"
	workspaces_entity "github.com/a-digi/coco-iam/src/admin/workspaces/entity"
	workspaces_admin "github.com/a-digi/coco-iam/src/admin/workspaces/handler"
)

var (
	apiResourceHandlerInstance *lift_api.ApiResourceHandler
	apiResourceHandlerOnce     sync.Once
)

func GetApiResourceHandler() *lift_api.ApiResourceHandler {
	apiResourceHandlerOnce.Do(func() {
		apiResourceHandlerInstance = &lift_api.ApiResourceHandler{
			EntityMap: map[string]lift_api.ResourceConfig{
				"users": {
					Entity: func() interface{} { return &entity.User{} },
					CustomHandlers: map[string]func(reqCtx request.RequestContext){
						http.MethodPost:   admin_users.CustomCreateUserHandler,
						http.MethodPatch:  admin_users.CustomUpdateUserHandler,
						http.MethodDelete: admin_users.CustomDeleteUserHandler,
					},
				},
				"admin_groups": {
					Entity: func() interface{} { return &admin_groups_entity.AdminGroup{} },
				},
				"admin_group_acl": {
					Entity: func() interface{} { return &acl_entity.AdminGroupAcl{} },
					CustomHandlers: map[string]func(reqCtx request.RequestContext){
						http.MethodPost:   admin_acl.CustomAdminGroupAclCreate,
						http.MethodGet:    admin_acl.CustomAdminGroupAclGet,
						http.MethodPatch:  admin_acl.CustomAdminGroupAclUpdate,
						http.MethodPut:    admin_acl.CustomAdminGroupAclUpdate,
						http.MethodDelete: admin_acl.CustomAdminGroupAclDelete,
					},
				},
				"admin_group_members": {
					Entity: func() interface{} { return &admin_groups_entity.AdminGroupMember{} },
				},
				"admin_acl": {
					Entity: func() interface{} { return &acl_entity.AdminAcl{} },
					CustomHandlers: map[string]func(reqCtx request.RequestContext){
						http.MethodPost:   admin_acl.CustomAdminUserAclCreate,
						http.MethodGet:    admin_acl.CustomAdminUserAclGet,
						http.MethodPatch:  admin_acl.CustomAdminUserAclUpdate,
						http.MethodPut:    admin_acl.CustomAdminUserAclUpdate,
						http.MethodDelete: admin_acl.CustomAdminUserAclDelete,
					},
				},
				"user_acl": {
					Entity: func() interface{} { return &acl_entity.UserAcl{} },
					CustomHandlers: map[string]func(reqCtx request.RequestContext){
						http.MethodPost:   organization_users_admin.CustomOrgUserBaseAclHandler,
						http.MethodGet:    organization_users_admin.CustomOrgUserBaseAclHandler,
						http.MethodPatch:  organization_users_admin.CustomOrgUserBaseAclHandler,
						http.MethodPut:    organization_users_admin.CustomOrgUserBaseAclHandler,
						http.MethodDelete: organization_users_admin.CustomOrgUserBaseAclHandler,
					},
				},
				"workspaces": {
					Entity: func() interface{} { return &workspaces_entity.Workspace{} },
					CustomHandlers: map[string]func(reqCtx request.RequestContext){
						http.MethodPost:   workspaces_admin.CustomWorkspacesHandler,
						http.MethodGet:    workspaces_admin.CustomWorkspacesHandler,
						http.MethodPatch:  workspaces_admin.CustomWorkspacesHandler,
						http.MethodPut:    workspaces_admin.CustomWorkspacesHandler,
						http.MethodDelete: workspaces_admin.CustomWorkspacesHandler,
					},
				},
				"organizations": {
					Entity: func() interface{} { return &organizations_entity.Organization{} },
					PostEventListener: []event.PostEventListener{
						&profile_listener.ProvisionOrgDBOnCreate{},
						&org_user_listener.ProvisionUsersDBOnCreate{},
						&apicred_listener.ProvisionApiCredentialsDBOnCreate{},
						&oauth_listener.ProvisionOAuthDBOnCreate{},
					},
					DeleteEventListener: []event.DeleteEventListener{
						&organization_deleted.OrganizationDeleteListener{},
					},
				},
				"organization_users": {
					Entity: func() interface{} { return &organization_users_entity.User{} },
					CustomHandlers: map[string]func(reqCtx request.RequestContext){
						http.MethodPost:   organization_users_admin.CustomCreateOrganizationUserHandler,
						http.MethodGet:    organization_users_admin.CustomGetOrganizationUsersHandler,
						http.MethodPatch:  organization_users_admin.CustomPatchOrganizationUserHandler,
						http.MethodPut:    organization_users_admin.CustomPatchOrganizationUserHandler,
						http.MethodDelete: organization_users_admin.CustomDeleteOrganizationUserHandler,
					},
					DeleteEventListener: []event.DeleteEventListener{
						&application_cleanup_producers.OrganizationUserDeleteListener{},
					},
				},
				"organization_groups": {
					Entity: func() interface{} { return &organization_groups_entity.OrganizationGroup{} },
					CustomHandlers: map[string]func(reqCtx request.RequestContext){
						http.MethodPost:   organization_groups_admin.CustomOrgGroupsHandler,
						http.MethodGet:    organization_groups_admin.CustomOrgGroupsHandler,
						http.MethodPatch:  organization_groups_admin.CustomOrgGroupsHandler,
						http.MethodPut:    organization_groups_admin.CustomOrgGroupsHandler,
						http.MethodDelete: organization_groups_admin.CustomOrgGroupsHandler,
					},
					DeleteEventListener: []event.DeleteEventListener{
						&application_cleanup_producers.OrganizationGroupDeleteListener{},
					},
				},
				"organization_group_members": {
					Entity: func() interface{} { return &organization_groups_entity.OrganizationGroupMember{} },
					CustomHandlers: map[string]func(reqCtx request.RequestContext){
						http.MethodPost:   organization_groups_admin.CustomOrgGroupMembersHandler,
						http.MethodGet:    organization_groups_admin.CustomOrgGroupMembersHandler,
						http.MethodPatch:  organization_groups_admin.CustomOrgGroupMembersHandler,
						http.MethodPut:    organization_groups_admin.CustomOrgGroupMembersHandler,
						http.MethodDelete: organization_groups_admin.CustomOrgGroupMembersHandler,
					},
					DeleteEventListener: []event.DeleteEventListener{
						&application_cleanup_producers.OrganizationGroupMemberDeleteListener{},
					},
				},
				"organization_user_acl": {
					Entity: func() interface{} { return &organization_users_entity.OrganizationUserAcl{} },
					CustomHandlers: map[string]func(reqCtx request.RequestContext){
						http.MethodPost:   organization_users_admin.CustomOrgUserAclHandler,
						http.MethodGet:    organization_users_admin.CustomOrgUserAclHandler,
						http.MethodPatch:  organization_users_admin.CustomOrgUserAclHandler,
						http.MethodPut:    organization_users_admin.CustomOrgUserAclHandler,
						http.MethodDelete: organization_users_admin.CustomOrgUserAclHandler,
					},
				},
				"organization_group_acl": {
					Entity: func() interface{} { return &organization_groups_entity.OrganizationGroupAcl{} },
					CustomHandlers: map[string]func(reqCtx request.RequestContext){
						http.MethodPost:   organization_groups_admin.CustomOrgGroupAclHandler,
						http.MethodGet:    organization_groups_admin.CustomOrgGroupAclHandler,
						http.MethodPatch:  organization_groups_admin.CustomOrgGroupAclHandler,
						http.MethodPut:    organization_groups_admin.CustomOrgGroupAclHandler,
						http.MethodDelete: organization_groups_admin.CustomOrgGroupAclHandler,
					},
				},
				"applications": {
					Entity: func() interface{} { return &applications_entity.Application{} },
					CustomHandlers: map[string]func(reqCtx request.RequestContext){
						http.MethodPost:   application_admin.CustomApplicationsHandler,
						http.MethodGet:    application_admin.CustomApplicationsHandler,
						http.MethodPatch:  application_admin.CustomApplicationsHandler,
						http.MethodPut:    application_admin.CustomApplicationsHandler,
						http.MethodDelete: application_admin.CustomApplicationsHandler,
					},
				},
				"application_scopes": {
					Entity: func() interface{} { return &applications_entity.ApplicationScope{} },
					CustomHandlers: map[string]func(reqCtx request.RequestContext){
						http.MethodPost:   application_admin.CustomApplicationScopesHandler,
						http.MethodGet:    application_admin.CustomApplicationScopesHandler,
						http.MethodPatch:  application_admin.CustomApplicationScopesHandler,
						http.MethodPut:    application_admin.CustomApplicationScopesHandler,
						http.MethodDelete: application_admin.CustomApplicationScopesHandler,
					},
				},
				"application_user_acl": {
					Entity: func() interface{} { return &applications_entity.ApplicationUserAcl{} },
					CustomHandlers: map[string]func(reqCtx request.RequestContext){
						http.MethodPost:   application_admin.CustomApplicationUserAclHandler,
						http.MethodGet:    application_admin.CustomApplicationUserAclHandler,
						http.MethodPatch:  application_admin.CustomApplicationUserAclHandler,
						http.MethodPut:    application_admin.CustomApplicationUserAclHandler,
						http.MethodDelete: application_admin.CustomApplicationUserAclHandler,
					},
				},
				"application_group_acl": {
					Entity: func() interface{} { return &applications_entity.ApplicationGroupAcl{} },
					CustomHandlers: map[string]func(reqCtx request.RequestContext){
						http.MethodPost:   application_admin.CustomApplicationGroupAclHandler,
						http.MethodGet:    application_admin.CustomApplicationGroupAclHandler,
						http.MethodPatch:  application_admin.CustomApplicationGroupAclHandler,
						http.MethodPut:    application_admin.CustomApplicationGroupAclHandler,
						http.MethodDelete: application_admin.CustomApplicationGroupAclHandler,
					},
				},
				// queue_tasks is no longer a generic resource — it has moved
				// out of main.db into per-queue files under
				// ./data/db/queue/<id>_<name>.db. Custom handlers
				// (AdminQueueTasksListHandler / AdminQueueTaskGetHandler)
				// fan out across the per-queue files for the admin UI;
				// the generic ApiResourceHandler can't address that layout.
			},
		}
	})

	return apiResourceHandlerInstance
}
