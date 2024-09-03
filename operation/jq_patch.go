package operation

// JQPatch
// doc: https://flant.github.io/shell-operator/KUBERNETES.html#jqpatch
type JQPatch struct {
	Operation  string `json:"operation"`
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Namespace  string `json:"namespace"`
	Name       string `json:"name"`
	JQFilter   string `json:"jqFilter"`
}
