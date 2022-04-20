package controller

type ResourceEventType string

const (
	ResourceAdded   ResourceEventType = "ResourceAdded"
	ResourceUpdated ResourceEventType = "ResourceUpdated"
	ResourceDeleted ResourceEventType = "ResourceDeleted"
)

type ResourceEvent struct {
	Obj    interface{}
	OldObj interface{}
	Type   ResourceEventType
}
