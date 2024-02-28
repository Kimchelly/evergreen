package route

import (
	"net/http"

	"github.com/evergreen-ci/evergreen"
	"github.com/evergreen-ci/evergreen/model"
	"github.com/evergreen-ci/evergreen/rest/data"
	"github.com/evergreen-ci/gimlet"
	"github.com/evergreen-ci/gimlet/acl"
	"github.com/mongodb/amboy"
)

const defaultLimit = 100

type HandlerOpts struct {
	APIQueue            amboy.Queue
	TaskDispatcher      model.TaskQueueItemDispatcher
	TaskAliasDispatcher model.TaskQueueItemDispatcher
	URL                 string
	GithubSecret        []byte
}

// AttachHandler attaches the api's request handlers to the given mux router.
// It builds a Connector then attaches each of the main functions for
// the api to the router.
func AttachHandler(app *gimlet.APIApp, opts HandlerOpts) {
	sc := &data.DBConnector{}
	env := evergreen.GetEnvironment()
	settings := env.Settings()
	parsleyURL := settings.Ui.ParsleyUrl

	sc.SetURL(opts.URL)

	// Middleware
	requireUser := gimlet.NewRequireAuthHandler()
	requireValidGithubPayload := NewGithubAuthMiddleware()
	requireValidSNSPayload := NewSNSAuthMiddleware()
	requireTask := NewTaskAuthMiddleware()
	requireTaskHost := NewTaskHostAuthMiddleware()
	requireHost := NewHostAuthMiddleware()
	requirePod := NewPodAuthMiddleware()
	requirePodOrHost := NewPodOrHostAuthMiddleWare()
	addProject := NewProjectContextMiddleware()
	requireProjectAdmin := NewProjectAdminMiddleware()
	requireRepoAdmin := NewRepoAdminMiddleware()
	requireCommitQueueItemOwner := NewCommitQueueItemOwnerMiddleware()
	createProject := NewCanCreateMiddleware()
	adminSettings := RequiresSuperUserPermission(evergreen.PermissionAdminSettings, evergreen.AdminSettingsEdit)
	createDistro := RequiresSuperUserPermission(evergreen.PermissionDistroCreate, evergreen.DistroCreate)
	editRoles := RequiresSuperUserPermission(evergreen.PermissionRoleModify, evergreen.RoleModify)
	viewTasks := RequiresProjectPermission(evergreen.PermissionTasks, evergreen.TasksView)
	editTasks := RequiresProjectPermission(evergreen.PermissionTasks, evergreen.TasksBasic)
	editAnnotations := RequiresProjectPermission(evergreen.PermissionAnnotations, evergreen.AnnotationsModify)
	viewAnnotations := RequiresProjectPermission(evergreen.PermissionAnnotations, evergreen.AnnotationsView)
	submitPatches := RequiresProjectPermission(evergreen.PermissionPatches, evergreen.PatchSubmit)
	viewProjectSettings := RequiresProjectPermission(evergreen.PermissionProjectSettings, evergreen.ProjectSettingsView)
	editProjectSettings := RequiresProjectPermission(evergreen.PermissionProjectSettings, evergreen.ProjectSettingsEdit)
	editDistroSettings := RequiresDistroPermission(evergreen.PermissionDistroSettings, evergreen.DistroSettingsEdit)
	removeDistroSettings := RequiresDistroPermission(evergreen.PermissionDistroSettings, evergreen.DistroSettingsAdmin)
	editHosts := RequiresDistroPermission(evergreen.PermissionHosts, evergreen.HostsEdit)

	app.AddWrapper(gimlet.WrapperMiddleware(allowCORS))

	// Agent protocol routes
	app.AddRoute("/agent/cedar_config").Version(2).Get().Wrap(requirePodOrHost).RouteHandler(makeAgentCedarConfig(settings.Cedar))
	app.AddRoute("/agent/setup").Version(2).Get().Wrap(requirePodOrHost).RouteHandler(makeAgentSetup(settings))
	app.AddRoute("/commit_queue/{patch_id}/additional").Version(2).Get().Wrap(requireTask).RouteHandler(makeCommitQueueAdditionalPatches())
	app.AddRoute("/commit_queue/{patch_id}/conclude_merge").Version(2).Post().Wrap(requireTask).RouteHandler(makeCommitQueueConcludeMerge())
	app.AddRoute("/distros/{distro_id}/ami").Version(2).Get().Wrap(requireTask).RouteHandler(makeGetDistroAMI())
	app.AddRoute("/hosts/{host_id}/agent/next_task").Version(2).Get().Wrap(requireHost).RouteHandler(makeHostAgentNextTask(env, opts.TaskDispatcher, opts.TaskAliasDispatcher))
	app.AddRoute("/hosts/{host_id}/task/{task_id}/end").Version(2).Post().Wrap(requireHost, requireTask).RouteHandler(makeHostAgentEndTask(env))
	app.AddRoute("/hosts/{host_id}/status").Version(2).Get().Wrap(requireTaskHost).RouteHandler(makeContainerStatusManager())
	app.AddRoute("/hosts/{host_id}/logs/output").Version(2).Get().Wrap(requireTaskHost).RouteHandler(makeContainerLogsRouteManager(false))
	app.AddRoute("/hosts/{host_id}/logs/error").Version(2).Get().Wrap(requireTaskHost).RouteHandler(makeContainerLogsRouteManager(true))
	app.AddRoute("/hosts/{task_id}/create").Version(2).Post().Wrap(requireTask).RouteHandler(makeHostCreateRouteManager(env))
	app.AddRoute("/hosts/{task_id}/list").Version(2).Get().Wrap(requireTask).RouteHandler(makeHostListRouteManager())
	app.AddRoute("/pods/{pod_id}/agent/next_task").Version(2).Get().Wrap(requirePod).RouteHandler(makePodAgentNextTask(env))
	app.AddRoute("/pods/{pod_id}/task/{task_id}/end").Version(2).Post().Wrap(requirePod, requireTask).RouteHandler(makePodAgentEndTask(env))
	app.AddRoute("/task/{task_id}/").Version(2).Get().Wrap(requireTask).RouteHandler(makeFetchTask())
	app.AddRoute("/task/{task_id}/display_task").Version(2).Get().Wrap(requireTask).RouteHandler(makeGetDisplayTaskHandler())
	app.AddRoute("/task/{task_id}/distro_view").Version(2).Get().Wrap(requireTask, requirePodOrHost).RouteHandler(makeGetDistroView())
	app.AddRoute("/task/{task_id}/downstreamParams").Version(2).Post().Wrap(requireTask).RouteHandler(makeSetDownstreamParams())
	app.AddRoute("/task/{task_id}/expansions_and_vars").Version(2).Get().Wrap(requireTask, requirePodOrHost).RouteHandler(makeGetExpansionsAndVars(settings))
	app.AddRoute("/task/{task_id}/files").Version(2).Post().Wrap(requireTask, requirePodOrHost).RouteHandler(makeAttachFiles())
	app.AddRoute("/task/{task_id}/generate").Version(2).Post().Wrap(requireTask).RouteHandler(makeGenerateTasksHandler(env))
	app.AddRoute("/task/{task_id}/generate").Version(2).Get().Wrap(requireTask).RouteHandler(makeGenerateTasksPollHandler())
	app.AddRoute("/task/{task_id}/new_push").Version(2).Post().Wrap(requireTask).RouteHandler(makeNewPush())
	app.AddRoute("/task/{task_id}/heartbeat").Version(2).Post().Wrap(requireTask, requirePodOrHost).RouteHandler(makeHeartbeat())
	app.AddRoute("/task/{task_id}/parser_project").Version(2).Get().Wrap(requireTask).RouteHandler(makeGetParserProject(env))
	app.AddRoute("/task/{task_id}/project_ref").Version(2).Get().Wrap(requireTask).RouteHandler(makeGetProjectRef())
	app.AddRoute("/task/{task_id}/pull_request").Version(2).Get().Wrap(requireTask).RouteHandler(makeAgentGetPullRequest(settings))
	app.AddRoute("/task/{task_id}/set_results_info").Version(2).Post().Wrap(requireTask).RouteHandler(makeSetTaskResultsInfoHandler())
	app.AddRoute("/task/{task_id}/start").Version(2).Post().Wrap(requireTask, requirePodOrHost).RouteHandler(makeStartTask(env))
	app.AddRoute("/task/{task_id}/test_logs").Version(2).Post().Wrap(requireTask, requirePodOrHost).RouteHandler(makeAttachTestLog(settings))
	app.AddRoute("/task/{task_id}/git/patch").Version(2).Get().Wrap(requireTask).RouteHandler(makeGitServePatch())
	app.AddRoute("/task/{task_id}/git/patchfile/{patchfile_id}").Version(2).Get().Wrap(requireTask).RouteHandler(makeGitServePatchFile())
	app.AddRoute("/task/{task_id}/installation_token/{owner}/{repo}").Version(2).Get().Wrap(requireTask).RouteHandler(makeCreateInstallationToken(env))
	app.AddRoute("/task/{task_id}/keyval/inc").Version(2).Post().Wrap(requireTask).RouteHandler(makeKeyvalPluginInc())
	app.AddRoute("/task/{task_id}/manifest/load").Version(2).Get().Wrap(requireTask).RouteHandler(makeManifestLoad(settings))
	app.AddRoute("/task/{task_id}/update_push_status").Version(2).Post().Wrap(requireTask).RouteHandler(makeUpdatePushStatus())
	app.AddRoute("/task/{task_id}/restart").Version(2).Post().Wrap(requireTask).RouteHandler(makeMarkTaskForRestart())
	app.AddRoute("/task/{task_id}/check_run").Version(2).Post().Wrap(requireTask).RouteHandler(makeCheckRun(settings))

	// REST v2 API Routes
	app.AddRoute("/").Version(2).Get().Wrap(requireUser).RouteHandler(makePlaceHolder())
	app.AddRoute("/admin/banner").Version(2).Get().Wrap(requireUser).RouteHandler(makeFetchAdminBanner())
	app.AddRoute("/admin/banner").Version(2).Post().Wrap(adminSettings).RouteHandler(makeSetAdminBanner())
	app.AddRoute("/admin/uiv2_url").Version(2).Get().Wrap(requireUser).RouteHandler(makeFetchAdminUIV2Url())
	app.AddRoute("/admin/events").Version(2).Get().Wrap(adminSettings).RouteHandler(makeFetchAdminEvents(opts.URL))
	app.AddRoute("/admin/spawn_hosts").Version(2).Get().Wrap(adminSettings).RouteHandler(makeFetchSpawnHostUsage())
	app.AddRoute("/admin/restart/versions").Version(2).Post().Wrap(adminSettings).RouteHandler(makeRestartRoute(evergreen.RestartVersions, nil))
	app.AddRoute("/admin/restart/tasks").Version(2).Post().Wrap(adminSettings).RouteHandler(makeRestartRoute(evergreen.RestartTasks, opts.APIQueue))
	app.AddRoute("/admin/revert").Version(2).Post().Wrap(adminSettings).RouteHandler(makeRevertRouteManager())
	app.AddRoute("/admin/service_flags").Version(2).Post().Wrap(adminSettings).RouteHandler(makeSetServiceFlagsRouteManager())
	app.AddRoute("/admin/settings").Version(2).Get().Wrap(adminSettings).RouteHandler(makeFetchAdminSettings())
	app.AddRoute("/admin/settings").Version(2).Post().Wrap(adminSettings).RouteHandler(makeSetAdminSettings())
	app.AddRoute("/admin/task_queue").Version(2).Delete().Wrap(adminSettings).RouteHandler(makeClearTaskQueueHandler())
	app.AddRoute("/admin/commit_queues").Version(2).Delete().Wrap(adminSettings).RouteHandler(makeClearCommitQueuesHandler())
	app.AddRoute("/admin/service_users").Version(2).Get().Wrap(adminSettings).RouteHandler(makeGetServiceUsers())
	app.AddRoute("/admin/service_users").Version(2).Post().Wrap(adminSettings).RouteHandler(makeUpdateServiceUser())
	app.AddRoute("/admin/service_users").Version(2).Delete().Wrap(adminSettings).RouteHandler(makeDeleteServiceUser())
	app.AddRoute("/alias/{name}").Version(2).Get().Wrap(requireUser).RouteHandler(makeFetchAliases())
	app.AddRoute("/auth").Version(2).Get().Wrap(requireUser).RouteHandler(&authPermissionGetHandler{})
	app.AddRoute("/builds/{build_id}").Version(2).Get().Wrap(viewTasks).RouteHandler(makeGetBuildByID(env))
	app.AddRoute("/builds/{build_id}").Version(2).Patch().Wrap(requireUser, editTasks).RouteHandler(makeChangeStatusForBuild())
	app.AddRoute("/builds/{build_id}/abort").Version(2).Post().Wrap(requireUser, editTasks).RouteHandler(makeAbortBuild())
	app.AddRoute("/builds/{build_id}/restart").Version(2).Post().Wrap(requireUser, editTasks).RouteHandler(makeRestartBuild())
	app.AddRoute("/builds/{build_id}/tasks").Version(2).Get().Wrap(requireUser, viewTasks).RouteHandler(makeFetchTasksByBuild(parsleyURL, opts.URL))
	app.AddRoute("/builds/{build_id}/annotations").Version(2).Get().Wrap(requireUser, viewAnnotations).RouteHandler(makeFetchAnnotationsByBuild())
	app.AddRoute("/commit_queue/{project_id}").Version(2).Get().Wrap(requireUser, viewTasks).RouteHandler(makeGetCommitQueueItems())
	app.AddRoute("/commit_queue/{patch_id}").Version(2).Delete().Wrap(requireUser, addProject, requireCommitQueueItemOwner, editTasks).RouteHandler(makeDeleteCommitQueueItems(env))
	app.AddRoute("/commit_queue/{patch_id}").Version(2).Put().Wrap(requireUser, addProject, requireCommitQueueItemOwner, editTasks).RouteHandler(makeCommitQueueEnqueueItem())
	app.AddRoute("/commit_queue/{patch_id}/message").Version(2).Get().Wrap(requireUser).RouteHandler(makecqMessageForPatch())
	app.AddRoute("/distros").Version(2).Get().Wrap(requireUser).RouteHandler(makeDistroRoute())
	app.AddRoute("/distros/settings").Version(2).Patch().Wrap(createDistro).RouteHandler(makeModifyDistrosSettings())
	app.AddRoute("/distros/{distro_id}").Version(2).Get().Wrap(editDistroSettings).RouteHandler(makeGetDistroByID())
	app.AddRoute("/distros/{distro_id}").Version(2).Patch().Wrap(editDistroSettings).RouteHandler(makePatchDistroByID())
	app.AddRoute("/distros/{distro_id}").Version(2).Delete().Wrap(removeDistroSettings).RouteHandler(makeDeleteDistroByID())
	app.AddRoute("/distros/{distro_id}").Version(2).Put().Wrap(createDistro).RouteHandler(makePutDistro())
	app.AddRoute("/distros/{distro_id}/execute").Version(2).Patch().Wrap(editHosts).RouteHandler(makeDistroExecute(env))
	app.AddRoute("/distros/{distro_id}/icecream_config").Version(2).Patch().Wrap(editHosts).RouteHandler(makeDistroIcecreamConfig(env))
	app.AddRoute("/distros/{distro_id}/setup").Version(2).Get().Wrap(editDistroSettings).RouteHandler(makeGetDistroSetup())
	app.AddRoute("/distros/{distro_id}/setup").Version(2).Patch().Wrap(editDistroSettings).RouteHandler(makeChangeDistroSetup())
	// client_urls is used by the agent monitor deploy job which does not pass in user info
	app.AddRoute("/distros/{distro_id}/client_urls").Version(2).Get().RouteHandler(makeGetDistroClientURLs(env))

	app.AddRoute("/hooks/github").Version(2).Post().Wrap(requireValidGithubPayload).RouteHandler(makeGithubHooksRoute(sc, opts.APIQueue, opts.GithubSecret, settings))
	app.AddRoute("/hooks/aws").Version(2).Post().Wrap(requireValidSNSPayload).RouteHandler(makeEC2SNS(env, opts.APIQueue))
	app.AddRoute("/hooks/aws/ecs").Version(2).Post().Wrap(requireValidSNSPayload).RouteHandler(makeECSSNS(env, opts.APIQueue))

	app.AddRoute("/host/filter").Version(2).Get().Wrap(requireUser).RouteHandler(makeFetchHostFilter())
	app.AddRoute("/host/start_processes").Version(2).Post().Wrap(requireUser).RouteHandler(makeHostStartProcesses(env))
	app.AddRoute("/host/get_processes").Version(2).Get().Wrap(requireUser).RouteHandler(makeHostGetProcesses(env))
	app.AddRoute("/hosts").Version(2).Get().Wrap(requireUser).RouteHandler(makeFetchHosts(opts.URL))
	app.AddRoute("/hosts").Version(2).Post().Wrap(requireUser).RouteHandler(makeSpawnHostCreateRoute(env))
	app.AddRoute("/hosts").Version(2).Patch().Wrap(requireUser).RouteHandler(makeChangeHostsStatuses())
	app.AddRoute("/hosts/{host_id}").Version(2).Get().Wrap(requireUser).RouteHandler(makeGetHostByID())
	app.AddRoute("/hosts/{host_id}").Version(2).Patch().Wrap(requireUser).RouteHandler(makeHostModifyRouteManager(env))
	app.AddRoute("/hosts/{host_id}/disable").Version(2).Post().Wrap(requireHost).RouteHandler(makeDisableHostHandler(env))
	app.AddRoute("/hosts/{host_id}/stop").Version(2).Post().Wrap(requireUser).RouteHandler(makeHostStopManager(env))
	app.AddRoute("/hosts/{host_id}/start").Version(2).Post().Wrap(requireUser).RouteHandler(makeHostStartManager(env))
	app.AddRoute("/hosts/{host_id}/change_password").Version(2).Post().Wrap(requireUser).RouteHandler(makeHostChangePassword(env))
	app.AddRoute("/hosts/{host_id}/extend_expiration").Version(2).Post().Wrap(requireUser).RouteHandler(makeExtendHostExpiration())
	app.AddRoute("/hosts/{host_id}/terminate").Version(2).Post().Wrap(requireUser).RouteHandler(makeTerminateHostRoute())
	app.AddRoute("/hosts/{host_id}/attach").Version(2).Post().Wrap(requireUser).RouteHandler(makeAttachVolume(env))
	app.AddRoute("/hosts/{host_id}/detach").Version(2).Post().Wrap(requireUser).RouteHandler(makeDetachVolume(env))
	app.AddRoute("/hosts/{host_id}/provisioning_options").Version(2).Get().Wrap(requireHost).RouteHandler(makeHostProvisioningOptionsGetHandler(env))
	app.AddRoute("/hosts/ip_address/{ip_address}").Version(2).Get().Wrap(requireUser).RouteHandler(makeGetHostByIpAddress())
	app.AddRoute("/volumes").Version(2).Get().Wrap(requireUser).RouteHandler(makeGetVolumes())
	app.AddRoute("/volumes").Version(2).Post().Wrap(requireUser).RouteHandler(makeCreateVolume(env))
	app.AddRoute("/volumes/{volume_id}").Version(2).Wrap(requireUser).Delete().RouteHandler(makeDeleteVolume(env))
	app.AddRoute("/volumes/{volume_id}").Version(2).Wrap(requireUser).Patch().RouteHandler(makeModifyVolume(env))
	app.AddRoute("/volumes/{volume_id}").Version(2).Get().Wrap(requireUser).RouteHandler(makeGetVolumeByID())
	app.AddRoute("/keys").Version(2).Get().Wrap(requireUser).RouteHandler(makeFetchKeys())
	app.AddRoute("/keys").Version(2).Post().Wrap(requireUser).RouteHandler(makeSetKey())
	app.AddRoute("/keys/{key_name}").Version(2).Delete().Wrap(requireUser).RouteHandler(makeDeleteKeys())
	app.AddRoute("/notifications/{type}").Version(2).Post().Wrap(requireUser).RouteHandler(makeNotification(env))
	app.AddRoute("/patches/{patch_id}").Version(2).Get().Wrap(requireUser, viewTasks).RouteHandler(makeFetchPatchByID())
	app.AddRoute("/patches/{patch_id}").Version(2).Patch().Wrap(requireUser, submitPatches).RouteHandler(makeChangePatchStatus(env))
	app.AddRoute("/patches/{patch_id}/abort").Version(2).Post().Wrap(requireUser, submitPatches).RouteHandler(makeAbortPatch())
	app.AddRoute("/patches/{patch_id}/configure").Version(2).Post().Wrap(requireUser, submitPatches).RouteHandler(makeSchedulePatchHandler(env))
	app.AddRoute("/patches/{patch_id}/raw").Version(2).Get().Wrap(requireUser, viewTasks).RouteHandler(makePatchRawHandler())
	app.AddRoute("/patches/{patch_id}/raw_modules").Version(2).Get().Wrap(requireUser, viewTasks).RouteHandler(makeModuleRawHandler())
	app.AddRoute("/patches/{patch_id}/restart").Version(2).Post().Wrap(requireUser, submitPatches).RouteHandler(makeRestartPatch())
	app.AddRoute("/patches/{patch_id}/merge_patch").Version(2).Put().Wrap(requireUser, addProject, submitPatches, requireCommitQueueItemOwner).RouteHandler(makeMergePatch(env))
	app.AddRoute("/pods").Version(2).Post().Wrap(adminSettings).RouteHandler(makePostPod(env))
	app.AddRoute("/pods/{pod_id}").Version(2).Get().Wrap(adminSettings).RouteHandler(makeGetPod(env))
	app.AddRoute("/pods/{pod_id}/provisioning_script").Version(2).Get().Wrap(requirePod).RouteHandler(makePodProvisioningScript(env))
	app.AddRoute("/projects").Version(2).Get().Wrap(requireUser).RouteHandler(makeFetchProjectsRoute(opts.URL))
	app.AddRoute("/projects/test_alias").Version(2).Get().Wrap(requireUser).RouteHandler(makeGetProjectAliasResultsHandler())
	app.AddRoute("/projects/{project_id}").Version(2).Delete().Wrap(requireUser, addProject, requireProjectAdmin, editProjectSettings).RouteHandler(makeDeleteProject())
	app.AddRoute("/projects/{project_id}").Version(2).Get().Wrap(requireUser, addProject, viewProjectSettings).RouteHandler(makeGetProjectByID())
	app.AddRoute("/projects/{project_id}").Version(2).Patch().Wrap(requireUser, addProject, requireProjectAdmin, editProjectSettings).RouteHandler(makePatchProjectByID(settings))
	app.AddRoute("/projects/{project_id}/attach_to_repo").Version(2).Post().Wrap(requireUser, addProject, requireProjectAdmin, editProjectSettings).RouteHandler(makeAttachProjectToRepoHandler())
	app.AddRoute("/projects/{project_id}/detach_from_repo").Version(2).Post().Wrap(requireUser, addProject, requireProjectAdmin, editProjectSettings).RouteHandler(makeDetachProjectFromRepoHandler())
	app.AddRoute("/projects/{project_id}/repotracker").Version(2).Post().Wrap(requireUser, addProject).RouteHandler(makeRunRepotrackerForProject())
	app.AddRoute("/projects/{project_id}").Version(2).Put().Wrap(requireUser, createProject).RouteHandler(makePutProjectByID(env))
	app.AddRoute("/projects/{project_id}/copy").Version(2).Post().Wrap(requireUser, addProject, requireProjectAdmin, editProjectSettings).RouteHandler(makeCopyProject(env))
	app.AddRoute("/projects/{project_id}/copy/variables").Version(2).Post().Wrap(requireUser, addProject, requireProjectAdmin, editProjectSettings).RouteHandler(makeCopyVariables())
	app.AddRoute("/projects/{project_id}/events").Version(2).Get().Wrap(requireUser, addProject, requireProjectAdmin, viewProjectSettings).RouteHandler(makeFetchProjectEvents(opts.URL))
	app.AddRoute("/projects/{project_id}/patches").Version(2).Get().Wrap(requireUser, viewTasks).RouteHandler(makePatchesByProjectRoute(opts.URL))
	app.AddRoute("/projects/{project_id}/recent_versions").Version(2).Get().Wrap(requireUser, viewTasks).RouteHandler(makeFetchProjectVersionsLegacy())
	app.AddRoute("/projects/{project_id}/revisions/{commit_hash}/tasks").Version(2).Get().Wrap(requireUser, viewTasks).RouteHandler(makeTasksByProjectAndCommitHandler(parsleyURL, opts.URL))
	app.AddRoute("/projects/{project_id}/task_reliability").Version(2).Get().Wrap(requireUser).RouteHandler(makeGetProjectTaskReliability(opts.URL))
	app.AddRoute("/projects/{project_id}/task_stats").Version(2).Get().Wrap(requireUser, viewTasks).RouteHandler(makeGetProjectTaskStats(opts.URL))
	app.AddRoute("/projects/{project_id}/versions").Version(2).Get().Wrap(requireUser, viewTasks).RouteHandler(makeGetProjectVersionsHandler(opts.URL))
	app.AddRoute("/projects/{project_id}/versions").Version(2).Patch().Wrap(requireUser, requireProjectAdmin).RouteHandler(makeModifyProjectVersionsHandler(opts.URL))
	app.AddRoute("/projects/{project_id}/tasks/{task_name}").Version(2).Get().Wrap(requireUser, viewTasks).RouteHandler(makeGetProjectTasksHandler(opts.URL))
	app.AddRoute("/projects/{project_id}/task_executions").Version(2).Get().Wrap(requireUser, viewTasks).RouteHandler(makeGetProjectTaskExecutionsHandler())
	app.AddRoute("/projects/{project_id}/patch_trigger_aliases").Version(2).Get().Wrap(requireUser, viewTasks).RouteHandler(makeFetchPatchTriggerAliases())
	app.AddRoute("/projects/{project_id}/parameters").Version(2).Get().Wrap(requireUser, viewTasks).RouteHandler(makeFetchParameters())
	app.AddRoute("/projects/variables/rotate").Version(2).Put().Wrap(requireUser, adminSettings).RouteHandler(makeProjectVarsPut())
	app.AddRoute("/permissions").Version(2).Get().Wrap(requireUser).RouteHandler(&permissionsGetHandler{})
	app.AddRoute("/repos/{repo_id}").Version(2).Get().Wrap(requireUser, viewProjectSettings).RouteHandler(makeGetRepoByID())
	app.AddRoute("/repos/{repo_id}").Version(2).Patch().Wrap(requireUser, requireRepoAdmin, editProjectSettings).RouteHandler(makePatchRepoByID(settings))
	app.AddRoute("/roles").Version(2).Get().Wrap(requireUser).RouteHandler(acl.NewGetAllRolesHandler(env.RoleManager()))
	app.AddRoute("/roles").Version(2).Post().Wrap(requireUser).RouteHandler(acl.NewUpdateRoleHandler(env.RoleManager()))
	app.AddRoute("/roles/{role_id}/users").Version(2).Get().Wrap(requireUser).RouteHandler(makeGetUsersWithRole())
	app.AddRoute("/scheduler/compare_tasks").Version(2).Post().Wrap(requireUser).RouteHandler(makeCompareTasksRoute())
	app.AddRoute("/status/cli_version").Version(2).Get().Wrap(requireUser).RouteHandler(makeFetchCLIVersionRoute(env))
	app.AddRoute("/status/hosts/distros").Version(2).Get().Wrap(requireUser).RouteHandler(makeHostStatusByDistroRoute())
	app.AddRoute("/status/notifications").Version(2).Get().Wrap(requireUser).RouteHandler(makeFetchNotifcationStatusRoute())
	app.AddRoute("/status/recent_tasks").Version(2).Get().Wrap(requireUser).RouteHandler(makeRecentTaskStatusHandler())
	app.AddRoute("/subscriptions").Version(2).Delete().Wrap(requireUser).RouteHandler(makeDeleteSubscription())
	app.AddRoute("/subscriptions").Version(2).Get().Wrap(requireUser).RouteHandler(makeFetchSubscription())
	app.AddRoute("/subscriptions").Version(2).Post().Wrap(requireUser).RouteHandler(makeSetSubscription())
	app.AddRoute("/tasks/{task_id}").Version(2).Get().Wrap(requireUser, viewTasks).RouteHandler(makeGetTaskRoute(parsleyURL, opts.URL))
	app.AddRoute("/tasks/{task_id}").Version(2).Patch().Wrap(requireUser, addProject, editTasks).RouteHandler(makeModifyTaskRoute())
	app.AddRoute("/tasks/{task_id}/annotations").Version(2).Get().Wrap(requireUser, viewAnnotations).RouteHandler(makeFetchAnnotationsByTask())
	app.AddRoute("/tasks/{task_id}/annotation").Version(2).Put().Wrap(requireUser, editAnnotations).RouteHandler(makePutAnnotationsByTask())
	app.AddRoute("/tasks/{task_id}/annotation").Version(2).Patch().Wrap(requireUser, editAnnotations).RouteHandler(makePatchAnnotationsByTask())
	app.AddRoute("/tasks/{task_id}/created_ticket").Version(2).Put().Wrap(requireUser, editAnnotations).RouteHandler(makeCreatedTicketByTask())
	app.AddRoute("/tasks/{task_id}/abort").Version(2).Post().Wrap(requireUser, editTasks).RouteHandler(makeTaskAbortHandler())
	app.AddRoute("/tasks/{task_id}/manifest").Version(2).Get().Wrap(viewTasks).RouteHandler(makeGetManifestHandler())
	app.AddRoute("/tasks/{task_id}/restart").Version(2).Post().Wrap(addProject, requireUser, editTasks).RouteHandler(makeTaskRestartHandler())
	app.AddRoute("/tasks/{task_id}/tests").Version(2).Get().Wrap(addProject, viewTasks).RouteHandler(makeFetchTestsForTask(env, sc))
	app.AddRoute("/tasks/{task_id}/tests/count").Version(2).Get().Wrap(addProject, viewTasks).RouteHandler(makeFetchTestCountForTask())
	app.AddRoute("/tasks/{task_id}/sync_path").Version(2).Get().Wrap(requireUser).RouteHandler(makeTaskSyncPathGetHandler())
	app.AddRoute("/tasks/{task_id}/generated_tasks").Version(2).Get().Wrap(requireUser, viewTasks).RouteHandler(makeGetGeneratedTasks())
	app.AddRoute("/tasks/{task_id}/build/TaskLogs").Version(2).Get().Wrap(requireUser, viewTasks).RouteHandler(makeGetTaskLogs(opts.URL))
	app.AddRoute("/tasks/{task_id}/build/TestLogs/{path}").Version(2).Get().Wrap(requireUser, viewTasks).RouteHandler(makeGetTestLogs(opts.URL))
	app.AddRoute("/task/sync_read_credentials").Version(2).Get().Wrap(requireUser).RouteHandler(makeTaskSyncReadCredentialsGetHandler())
	app.AddRoute("/user/settings").Version(2).Get().Wrap(requireUser).RouteHandler(makeFetchUserConfig())
	app.AddRoute("/user/settings").Version(2).Post().Wrap(requireUser).RouteHandler(makeSetUserConfig())
	app.AddRoute("/users/{user_id}/hosts").Version(2).Get().Wrap(requireUser).RouteHandler(makeFetchHosts(opts.URL))
	app.AddRoute("/users/{user_id}/patches").Version(2).Get().Wrap(requireUser).RouteHandler(makeUserPatchHandler(opts.URL))
	app.AddRoute("/users/offboard_user").Version(2).Post().Wrap(requireUser, editRoles).RouteHandler(makeOffboardUser(env))
	app.AddRoute("/users/rename_user").Version(2).Post().Wrap(requireUser, editRoles).RouteHandler(makeRenameUser(env))
	app.AddRoute("/users/{user_id}/permissions").Version(2).Get().Wrap(requireUser).RouteHandler(makeGetUserPermissions(env.RoleManager()))
	app.AddRoute("/users/{user_id}/permissions").Version(2).Post().Wrap(requireUser, editRoles).RouteHandler(makeModifyUserPermissions(env.RoleManager()))
	app.AddRoute("/users/{user_id}/permissions").Version(2).Delete().Wrap(requireUser, editRoles).RouteHandler(makeDeleteUserPermissions(env.RoleManager()))
	app.AddRoute("/users/{user_id}/roles").Version(2).Post().Wrap(requireUser, editRoles).RouteHandler(makeModifyUserRoles(env.RoleManager()))
	app.AddRoute("/users/permissions").Version(2).Get().Wrap(requireUser).RouteHandler(makeGetAllUsersPermissions(env.RoleManager()))
	app.AddRoute("/versions").Version(2).Put().Wrap(requireUser).RouteHandler(makeVersionCreateHandler(sc))
	app.AddRoute("/versions/{version_id}").Version(2).Get().Wrap(viewTasks).RouteHandler(makeGetVersionByID())
	app.AddRoute("/versions/{version_id}").Version(2).Patch().Wrap(requireUser, editTasks).RouteHandler(makePatchVersion())
	app.AddRoute("/versions/{version_id}/abort").Version(2).Post().Wrap(requireUser, editTasks).RouteHandler(makeAbortVersion())
	app.AddRoute("/versions/{version_id}/builds").Version(2).Get().Wrap(requireUser, viewTasks).RouteHandler(makeGetVersionBuilds(env))
	app.AddRoute("/versions/{version_id}/restart").Version(2).Post().Wrap(requireUser, editTasks).RouteHandler(makeRestartVersion())
	app.AddRoute("/versions/{version_id}/annotations").Version(2).Get().Wrap(requireUser, viewAnnotations).RouteHandler(makeFetchAnnotationsByVersion())

	// Add an options method to every POST request to handle pre-flight Options requests.
	// These requests must not check for credentials and just validate whether a route exists
	// And allows requests from a origin.
	for _, route := range app.Routes() {
		if route.HasMethod(http.MethodPost) {
			app.AddRoute(route.GetRoute()).Version(2).Options().RouteHandler(makeOptionsHandler())
		}
	}
}
