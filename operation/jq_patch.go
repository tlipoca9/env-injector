package operation

type Operation struct {
	Operation  string `json:"operation"`
	APIVersion string `json:"apiVersion,omitempty"`
	Kind       string `json:"kind,omitempty"`
	Namespace  string `json:"namespace,omitempty"`
	Name       string `json:"name,omitempty"`
	JQFilter   string `json:"jqFilter,omitempty"`
}

// JQPatch
// doc: https://flant.github.io/shell-operator/KUBERNETES.html#jqpatch
type JQPatch struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Namespace  string `json:"namespace"`
	Name       string `json:"name"`
	JQFilter   string `json:"jqFilter"`
}

func (o *JQPatch) Render() *Operation {
	return &Operation{
		Operation:  "JQPatch",
		APIVersion: o.APIVersion,
		Kind:       o.Kind,
		Namespace:  o.Namespace,
		Name:       o.Name,
		JQFilter:   o.JQFilter,
	}
}
