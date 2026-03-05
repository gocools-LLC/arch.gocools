package model

type Resource struct {
	ID       string
	Type     string
	Provider string
	Region   string
	ARN      string
	Name     string
	State    string
	Tags     map[string]string
	Metadata map[string]string
}
