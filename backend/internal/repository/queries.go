package repository

import _ "embed"

//go:embed sql/area_insert.sql
var queryInsertArea string

//go:embed sql/area_update.sql
var queryUpdateArea string

//go:embed sql/area_get.sql
var queryGetArea string

//go:embed sql/area_list.sql
var queryListAreas string

//go:embed sql/job_get.sql
var queryGetJob string

//go:embed sql/job_insert.sql
var queryInsertJob string

//go:embed sql/job_delete.sql
var queryDeleteJob string

//go:embed sql/job_list_area.sql
var queryListJobsByArea string

//go:embed sql/job_lock.sql
var queryLockJob string

//go:embed sql/job_area.sql
var queryJobArea string

//go:embed sql/job_active_area.sql
var queryActiveJobsByArea string

//go:embed sql/job_running.sql
var querySetJobRunning string

//go:embed sql/job_stage.sql
var querySetJobStage string

//go:embed sql/job_failed.sql
var querySetJobFailed string

//go:embed sql/job_cancelled.sql
var querySetJobCancelled string

//go:embed sql/job_cancel_area.sql
var queryCancelAreaJobs string

//go:embed sql/job_input_revision.sql
var querySetJobInputRevision string

//go:embed sql/job_completed.sql
var querySetJobCompleted string

//go:embed sql/jobs_recover.sql
var queryRecoverJobs string

//go:embed sql/area_lock.sql
var queryLockArea string

//go:embed sql/area_clear_active.sql
var queryClearActiveJob string

//go:embed sql/area_set_active.sql
var querySetActiveJob string

//go:embed sql/area_delete.sql
var queryDeleteArea string

//go:embed sql/area_recover.sql
var queryRecoverAreas string

//go:embed sql/result_get.sql
var queryGetResult string

//go:embed sql/result_lock.sql
var queryLockResult string

//go:embed sql/result_delete_area.sql
var queryDeleteAreaResults string

//go:embed sql/result_insert.sql
var queryInsertResult string

//go:embed sql/area_publish_result.sql
var queryPublishResult string
