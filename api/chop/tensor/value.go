package tensor

import "math"

// Value represents a node in the computation graph.
type Value struct {
	Data       float64
	Grad       float64
	children   []*Value
	localGrads []float64
}

// NewValue creates a new Value node.
func NewValue(data float64, children []*Value, localGrads []float64) *Value {
	return &Value{
		Data:       data,
		Grad:       0,
		children:   children,
		localGrads: localGrads,
	}
}

func (v *Value) Add(other *Value) *Value {
	return NewValue(v.Data+other.Data, []*Value{v, other}, []float64{1, 1})
}

func (v *Value) Mul(other *Value) *Value {
	return NewValue(v.Data*other.Data, []*Value{v, other}, []float64{other.Data, v.Data})
}

func (v *Value) Pow(other float64) *Value {
	return NewValue(math.Pow(v.Data, other), []*Value{v}, []float64{other * math.Pow(v.Data, other-1)})
}

func (v *Value) Log() *Value {
	return NewValue(math.Log(v.Data), []*Value{v}, []float64{1 / v.Data})
}

func (v *Value) Exp() *Value {
	return NewValue(math.Exp(v.Data), []*Value{v}, []float64{math.Exp(v.Data)})
}

func (v *Value) Relu() *Value {
	out := math.Max(0, v.Data)
	localGrad := 0.0
	if v.Data > 0 {
		localGrad = 1.0
	}
	return NewValue(out, []*Value{v}, []float64{localGrad})
}

func (v *Value) Neg() *Value {
	return v.Mul(NewValue(-1, nil, nil))
}

func (v *Value) Sub(other *Value) *Value {
	return v.Add(other.Neg())
}

func (v *Value) TrueDiv(other *Value) *Value {
	return v.Mul(other.Pow(-1))
}

func (v *Value) Backward() {
	topo := []*Value{}
	visited := make(map[*Value]bool)

	var buildTopo func(node *Value)
	buildTopo = func(node *Value) {
		if !visited[node] {
			visited[node] = true
			for _, child := range node.children {
				buildTopo(child)
			}
			topo = append(topo, node)
		}
	}
	buildTopo(v)

	v.Grad = 1.0
	for i := len(topo) - 1; i >= 0; i-- {
		node := topo[i]
		for j, child := range node.children {
			child.Grad += node.localGrads[j] * node.Grad
		}
	}
}
