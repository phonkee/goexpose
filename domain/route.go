package domain

type Route struct {
	Authorizers    Authorizers
	Method         string
	Path           string
	TaskConfig     *TaskConfig
	EndpointConfig *EndpointConfig
	Task           Task
}
