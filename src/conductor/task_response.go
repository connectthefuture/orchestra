// task_response.go
//
package main

import (
	o "orchestra"
)

type TaskResponse struct {
	id       uint64
	State    ResponseState     `json:"state"`
	Response map[string]string `json:"response"`
}

// Response related magic

func NewTaskResponse() (resp *TaskResponse) {
	resp = new(TaskResponse)
	resp.Response = make(map[string]string)

	return resp
}

func (resp *TaskResponse) IsFinished() bool {
	return resp.State.Finished()
}

func (resp *TaskResponse) DidFail() bool {
	return resp.State.Failed()
}

func (resp *TaskResponse) CanRetry() bool {
	return resp.State.CanRetry()
}

func ResponseFromProto(ptr *o.ProtoTaskResponse) (r *TaskResponse) {
	r = new(TaskResponse)

	switch *(ptr.Status) {
	case o.ProtoTaskResponse_JOB_INPROGRESS:
		r.State = RESP_RUNNING
	case o.ProtoTaskResponse_JOB_SUCCESS:
		r.State = RESP_FINISHED
	case o.ProtoTaskResponse_JOB_FAILED:
		r.State = RESP_FAILED
	case o.ProtoTaskResponse_JOB_HOST_FAILURE:
		r.State = RESP_FAILED_HOST_ERROR
	case o.ProtoTaskResponse_JOB_UNKNOWN:
		r.State = RESP_FAILED_UNKNOWN_SCORE
	case o.ProtoTaskResponse_JOB_UNKNOWN_FAILURE:
		fallthrough
	default:
		r.State = RESP_FAILED_UNKNOWN
	}

	r.id = *(ptr.Id)
	r.Response = o.MapFromProtoJobParameters(ptr.Response)

	return r
}
