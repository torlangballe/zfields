package zfields

import (
	"github.com/torlangballe/zui"
	"github.com/torlangballe/zutil/zgeo"
)

type FieldView struct {
	zui.StackView
	fieldOwner
	parentField *Field
}

func fieldViewNew(id string, vertical bool, structure interface{}, spacing float64, marg zgeo.Size, labelizeWidth float64) *FieldView {
	v := &FieldView{}
	v.StackView.Init(v, id)
	v.SetSpacing(12)
	v.SetMargin(zgeo.RectFromMinMax(marg.Pos(), marg.Pos().Negative()))
	v.Vertical = vertical
	v.structure = structure
	v.fieldOwner.labelizeWidth = labelizeWidth
	v.fieldOwner.id = id
	v.getSubStruct = func(structID string) interface{} {
		return v.structure
	}
	children := v.getStructItems("", true)
	// zlog.Info("FieldViewNew", id, len(children), labelizeWidth)
	for i, item := range children {
		var f Field
		if f.makeFromReflectItem(&v.fieldOwner, structure, item, i) {
			v.fields = append(v.fields, f)
		}
	}
	return v
}

func (v *FieldView) Build(update bool) {
	v.buildStack(v.ObjectName(), &v.StackView, v.parentField, &v.fields, zgeo.Left|zgeo.Top, zgeo.Size{}, true, 5, "") // Size{6, 4}
	if update {
		v.Update()
	}
}

func (v *FieldView) Update() {
	// fmt.Printf("FV Update: %+v\n", v.structure)
	v.fieldOwner.updateStack(&v.StackView, "")
}

func FieldViewNew(id string, structure interface{}, labelizeWidth float64) *FieldView {
	v := fieldViewNew(id, true, structure, 12, zgeo.Size{10, 10}, labelizeWidth)
	return v
}
