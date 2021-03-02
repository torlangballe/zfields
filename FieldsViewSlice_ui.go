// +build zui

package zfields

import (
	"fmt"
	"reflect"

	"github.com/torlangballe/zui"
	"github.com/torlangballe/zutil/zgeo"
	"github.com/torlangballe/zutil/zint"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zreflect"
	"github.com/torlangballe/zutil/zslice"
)

func makeCircledButton() *zui.ShapeView {
	v := zui.ShapeViewNew(zui.ShapeViewTypeCircle, zgeo.Size{30, 30})
	v.SetColor(zgeo.ColorNewGray(0.8, 1))
	return v
}

func makeCircledTextButton(text string, f *Field) *zui.ShapeView {
	v := makeCircledButton()
	w := zui.FontDefaultSize + 6
	font := zui.FontNice(w, zui.FontStyleNormal)
	f.SetFont(v, font)
	v.SetText(text)
	v.SetTextColor(zgeo.ColorBlack)
	v.SetTextAlignment(zgeo.Center)
	return v
}

func makeCircledImageButton(iname string) *zui.ShapeView {
	v := makeCircledButton()
	v.ImageMaxSize = zgeo.Size{20, 20}
	v.SetImage(nil, "images/"+iname+".png", nil)
	return v
}

func makeCircledTrashButton() *zui.ShapeView {
	trash := makeCircledImageButton("trash")
	trash.SetColor(zgeo.ColorNew(1, 0.8, 0.8, 1))
	return trash
}

func (v *FieldView) updateSliceValue(structure interface{}, stack *zui.StackView, vertical bool, f *Field, sendEdited bool) zui.View {
	ct := stack.Parent().View.(zui.ContainerType)
	newStack := v.buildStackFromSlice(structure, vertical, f)
	ct.ReplaceChild(stack, newStack)
	ns := zui.ViewGetNative(newStack)
	ctp := ns.Parent().Parent().View.(zui.ContainerType)
	ctp.ArrangeChildren(nil)
	if sendEdited {
		fh, _ := structure.(ActionHandler)
		// zlog.Info("updateSliceValue:", fh != nil, f.Name, fh)
		if fh != nil {
			fh.HandleAction(f, EditedAction, &newStack)
		}
	}
	return newStack
}

func (v *FieldView) makeNamedSelectionKey(f *Field) string {
	// zlog.Info("makeNamedSelectKey:", v.id, f.FieldName)
	return v.id + "." + f.FieldName + ".NamedSelectionIndex"
}

func (v *FieldView) changeNamedSelectionIndex(i int, f *Field) {
	key := v.makeNamedSelectionKey(f)
	zui.DefaultLocalKeyValueStore.SetInt(i, key, true)
}

func (v *FieldView) buildStackFromSlice(structure interface{}, vertical bool, f *Field) zui.View {
	sliceVal, _ := zreflect.FindFieldWithNameInStruct(f.FieldName, structure, true)
	// fmt.Printf("buildStackFromSlice: %s %v %v %+v\n", f.FieldName, reflect.ValueOf(structure).Kind(), reflect.ValueOf(structure).Type(), structure)
	var bar *zui.StackView
	stack := zui.StackViewNew(vertical, f.ID)
	if f != nil && f.Spacing != 0 {
		stack.SetSpacing(f.Spacing)
	}
	key := v.makeNamedSelectionKey(f)
	var selectedIndex int
	single := (f.Flags&flagIsNamedSelection != 0)
	var fieldView *FieldView
	// zlog.Info("buildStackFromSlice:", vertical, f.ID, val.Len())
	if single {
		selectedIndex, _ = zui.DefaultLocalKeyValueStore.GetInt(key, 0)
		// zlog.Info("buildStackFromSlice:", key, selectedIndex, vertical, f.ID)
		zint.Minimize(&selectedIndex, sliceVal.Len()-1)
		zint.Maximize(&selectedIndex, 0)
		stack.SetMargin(zgeo.RectFromXY2(8, 6, -8, -10))
		stack.SetCorner(zui.GroupingStrokeCorner)
		stack.SetStroke(zui.GroupingStrokeWidth, zui.GroupingStrokeColor)
		label := zui.LabelNew(f.Name)
		label.SetColor(zgeo.ColorNewGray(0, 1))
		font := zui.FontNice(zui.FontDefaultSize, zui.FontStyleBoldItalic)
		f.SetFont(label, font)
		stack.Add(zgeo.TopLeft, label)
	}
	for n := 0; n < sliceVal.Len(); n++ {
		var view zui.View
		nval := sliceVal.Index(n)
		h, _ := nval.Interface().(ActionFieldHandler)
		if h != nil {
			if h.HandleFieldAction(f, CreateFieldViewAction, &view) {
				stack.Add(zgeo.TopLeft, view)
			}
		}
		if view == nil {
			childStruct := nval.Addr().Interface()
			vert := !vertical
			if f.LabelizeWidth != 0 {
				vert = true
			}
			// fmt.Printf("buildStackFromSlice element: %s %p\n", f.FieldName, childStruct)

			fieldView = fieldViewNew(f.ID, vert, childStruct, 10, zgeo.Size{}, v.labelizeWidth, v)
			view = fieldView
			fieldView.parentField = f
			a := zgeo.Left //| zgeo.HorExpand
			if fieldView.Vertical {
				a |= zgeo.Top
			} else {
				a |= zgeo.VertCenter
			}

			fieldView.buildStack(f.ID, a, zgeo.Size{}, true, 5)
			if !f.IsStatic() && !single {
				trash := makeCircledTrashButton()
				trash.SetColor(zgeo.ColorNew(1, 0.8, 0.8, 1))
				fieldView.Add(zgeo.CenterLeft, trash)
				index := n
				trash.SetPressedHandler(func() {
					val, _ := zreflect.FindFieldWithNameInStruct(f.FieldName, structure, true)
					zslice.RemoveAt(val.Addr().Interface(), index)
					// zlog.Info("newlen:", index, val.Len())
					v.updateSliceValue(structure, stack, vertical, f, true)
				})
			}
			stack.Add(zgeo.TopLeft|zgeo.HorExpand, fieldView)
		}
		collapse := single && n != selectedIndex
		stack.CollapseChild(view, collapse, false)
	}
	if single {
		zlog.Assert(!f.IsStatic())
		bar = zui.StackViewHor(f.ID + ".bar")
		stack.Add(zgeo.TopLeft|zgeo.HorExpand, bar)
	}
	if !f.IsStatic() {
		plus := makeCircledImageButton("plus")
		plus.SetPressedHandler(func() {
			val, _ := zreflect.FindFieldWithNameInStruct(f.FieldName, structure, true)
			a := reflect.New(val.Type().Elem()).Elem()
			nv := reflect.Append(val, a)
			if fieldView != nil {
				fieldView.structure = nv.Interface()
			}
			val.Set(nv)
			a = val.Index(val.Len() - 1) // we re-set a, as it is now a new value at the end of slice
			if single {
				v.changeNamedSelectionIndex(val.Len()-1, f)
			}
			// fmt.Printf("SLICER + Pressed: %p %p\n", val.Index(val.Len()-1).Addr().Interface(), a.Addr().Interface())
			fhItem, _ := a.Addr().Interface().(ActionHandler)
			if fhItem != nil {
				fhItem.HandleAction(f, NewStructAction, nil)
			}
			v.updateSliceValue(structure, stack, vertical, f, true)
			//			stack.CustomView.PressedHandler()()
		})
		if bar != nil {
			bar.Add(zgeo.CenterRight, plus)
		} else {
			stack.Add(zgeo.TopLeft, plus)
		}
	}
	if single {
		shape := makeCircledTrashButton()
		bar.Add(zgeo.CenterRight, shape)
		shape.SetPressedHandler(func() {
			zui.AlertAsk("Delete this entry?", func(ok bool) {
				if ok {
					val, _ := zreflect.FindFieldWithNameInStruct(f.FieldName, structure, true)
					zslice.RemoveAt(val.Addr().Interface(), selectedIndex)
					// zlog.Info("newlen:", index, val.Len())
					v.updateSliceValue(structure, stack, vertical, f, true)
				}
			})
		})
		shape.SetUsable(sliceVal.Len() > 0)
		// zlog.Info("Make Slice thing:", key, selectedIndex, val.Len())

		shape = makeCircledTextButton("⇦", f)
		bar.Add(zgeo.CenterLeft, shape)
		shape.SetPressedHandler(func() {
			v.changeNamedSelectionIndex(selectedIndex-1, f)
			v.updateSliceValue(structure, stack, vertical, f, false)
		})
		shape.SetUsable(selectedIndex > 0)

		str := "0"
		if sliceVal.Len() > 0 {
			str = fmt.Sprintf("%d of %d", selectedIndex+1, sliceVal.Len())
		}
		label := zui.LabelNew(str)
		label.SetColor(zgeo.ColorNewGray(0, 1))
		f.SetFont(label, nil)
		bar.Add(zgeo.CenterLeft, label)

		shape = makeCircledTextButton("⇨", f)
		bar.Add(zgeo.CenterLeft, shape)
		shape.SetPressedHandler(func() {
			v.changeNamedSelectionIndex(selectedIndex+1, f)
			v.updateSliceValue(structure, stack, vertical, f, false)
		})
		shape.SetUsable(selectedIndex < sliceVal.Len()-1)
	}
	return stack
}

func updateSliceFieldView(view zui.View, item zreflect.Item, f *Field) {
	// zlog.Info("updateSliceFieldView:", view.ObjectName(), item.FieldName, f.Name)
	children := (view.(zui.ContainerType)).GetChildren()
	n := 0
	subViewCount := len(children)
	single := (f.Flags&flagIsNamedSelection != 0)
	if single {
		subViewCount -= 2
	}
	// if subViewCount != item.Value.Len() {
	// 	zlog.Info("SLICE VIEW: length changed!!!", subViewCount, item.Value.Len())
	// }
	for _, c := range children {
		if n >= item.Value.Len() {
			break
		}
		cview := c
		fv, _ := c.(*FieldView)
		// zlog.Info("CHILD:", c.ObjectName(), fv != nil, reflect.ValueOf(c).Type())
		val := item.Value.Index(n)
		if fv == nil {
			ah, _ := val.Interface().(ActionFieldHandler)
			if ah != nil {
				ah.HandleFieldAction(f, DataChangedAction, &cview)
				n++
				continue
			}
		} else {
			// fmt.Printf("Update Sub Slice field: %s %+v\n", fv.ObjectName(), val.Addr().Interface())
			n++
			fv.structure = val.Addr().Interface()
			fv.Update()
		}
		// zlog.Info("struct make field view:", f.Name, f.Kind, exp)
	}
	// if updateStackFromActionFieldHandlerSlice(view, &item, f) {
	// 	continue
	// }
}
