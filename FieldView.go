package zfields

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/torlangballe/zui"
	"github.com/torlangballe/zutil/zbool"
	"github.com/torlangballe/zutil/zdict"
	"github.com/torlangballe/zutil/zfloat"
	"github.com/torlangballe/zutil/zgeo"
	"github.com/torlangballe/zutil/zint"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zreflect"
	"github.com/torlangballe/zutil/zslice"
	"github.com/torlangballe/zutil/zstr"
	"github.com/torlangballe/zutil/ztime"
	"github.com/torlangballe/zutil/ztimer"
)

type FieldView struct {
	zui.StackView
	parent      *FieldView
	fields      []Field
	parentField *Field
	structure   interface{} // structure of ALL, not just a row
	changed     bool
	//	oldStructure  interface{}
	id            string
	handleUpdate  func(edited bool)
	labelizeWidth float64
	//	getSubStruct  func(structID string, direct bool) interface{}
}

func (v *FieldView) Struct() interface{} {
	return v.structure
}

func fieldViewNew(id string, vertical bool, structure interface{}, spacing float64, marg zgeo.Size, labelizeWidth float64, parent *FieldView) *FieldView {
	// start := time.Now()
	v := &FieldView{}
	v.StackView.Init(v, vertical, id)
	// zlog.Info("fieldViewNew", reflect.ValueOf(v.View).Type())

	v.SetSpacing(6)
	v.SetMargin(zgeo.RectFromMinMax(marg.Pos(), marg.Pos().Negative()))
	v.structure = structure
	v.labelizeWidth = labelizeWidth
	v.id = id
	v.parent = parent
	children := v.getStructItems()
	// zlog.Info("FieldViewNew", id, len(children), labelizeWidth)
	for i, item := range children {
		var f Field
		if f.makeFromReflectItem(structure, item, i) {
			v.fields = append(v.fields, f)
		}
	}
	return v
}

func (v *FieldView) Build(update bool) {
	a := zgeo.Left //| zgeo.HorExpand
	if v.Vertical {
		a |= zgeo.Top
	} else {
		a |= zgeo.VertCenter
	}
	v.buildStack(v.ObjectName(), a, zgeo.Size{}, true, 5) // Size{6, 4}
	if update {
		v.Update()
	}
}

func (v *FieldView) findNamedViewOrInLabelized(name string) zui.View {
	for _, c := range (v.View.(zui.ContainerType)).GetChildren() {
		n := c.ObjectName()
		if n == name {
			return c
		}
		if strings.HasPrefix(n, "$labelize.") {
			s := c.(*zui.StackView)
			v := s.FindViewWithName(name, false)
			if v != nil {
				return v
			}
		}
	}
	return nil
}

func (v *FieldView) Update() {
	children := v.getStructItems()
	// fmt.Println("FV Update", v.id, len(children))
	// fmt.Printf("FV Update: %s %d %+v\n", v.id, len(children), v.structure)
	for i, item := range children {
		f := findFieldWithIndex(&v.fields, i)
		if f == nil {
			// zlog.Info("FV Update no index found:", i, v.id)
			continue
		}
		// fmt.Println("FV Update Item:", f.Name)
		fview := v.findNamedViewOrInLabelized(f.ID)
		if fview == nil {
			//			zlog.Info("FV Update no view found:", i, v.id, f.ID)
			continue
		}
		called := v.callActionHandlerFunc(f, DataChangedAction, item.Interface, &fview)
		if called {
			continue
		}
		// fmt.Println("FV Update Item2:", f.Name)

		menu, _ := fview.(*zui.MenuView)
		if (f.Enum != "" && f.Kind != zreflect.KindSlice) || f.LocalEnum != "" {
			var enum zdict.Items
			if f.Enum != "" {
				enum, _ = fieldEnums[f.Enum]
				// zlog.Info("UpdateStack Enum:", f.Name)
				// zdict.DumpNamedValues(enum)
			} else {
				ei := findLocalField(&children, f.LocalEnum)
				zlog.Assert(ei != nil, f.Name, f.LocalEnum)
				enum = ei.Interface.(zdict.ItemsGetter).GetItems()
			}
			// zlog.Assert(enum != nil, f.Name, f.LocalEnum, f.Enum)
			menu.SetAndSelect(enum, item.Interface)
			continue
		}
		if f.LocalEnable != "" {
			eItem := findLocalField(&children, f.LocalEnable)
			e, got := eItem.Interface.(bool)
			// zlog.Info("updateStack localEnable:", f.Name, f.LocalEnable, e, got)
			if got {
				parent := zui.ViewGetNative(fview).Parent()
				//				if parent != nil && parent != stack(strings.HasPrefix(parent.ObjectName(), "$labelize.") || strings.HasPrefix(parent.ObjectName(), "$labledCheckBoxStack.")) {
				if parent != nil && parent != &v.NativeView {
					parent.SetUsable(e)
				} else {
					fview.SetUsable(e)
				}
			}
		}
		// if f.Flags&flagIsMenuedGroup != 0 {
		// 	updateMenuedGroup(view, item)
		// 	continue
		// }
		if f.Flags&flagIsButton != 0 {
			enabled, is := item.Interface.(bool)
			if is {
				fview.SetUsable(enabled)
			}
			continue
		}
		if menu == nil && f.Kind == zreflect.KindSlice {
			// val, found := zreflect.FindFieldWithNameInStruct(f.FieldName, v.structure, true)
			// fmt.Printf("updateSliceFieldView: %s %p %p %v %p\n", v.id, item.Interface, val.Interface(), found, view)
			updateSliceFieldView(fview, item, f)
		}

		switch f.Kind {
		case zreflect.KindSlice:
			getter, _ := item.Interface.(zdict.ItemsGetter)
			if getter != nil {
				items := getter.GetItems()
				menu := fview.(*zui.MenuView)
				menu.SetValues(items)
			}
		case zreflect.KindTime:
			tv, _ := fview.(*zui.TextView)
			if tv != nil && tv.IsEditing() {
				break
			}
			if f.Flags&flagIsDuration != 0 {
				v.updateSinceTime(fview.(*zui.Label), f)
				break
			}
			str := getTimeString(item, f)
			to := fview.(zui.TextLayoutOwner)
			to.SetText(str)

		case zreflect.KindStruct:
			_, got := item.Interface.(zui.UIStringer)
			if got {
				break
			}
			fv, _ := fview.(*FieldView)
			if fv == nil {
				break
			}
			//			fmt.Printf("Update substruct: %s %p %p\n", v.id, v.structure, item.Address)
			//			fv.SetStructure(item.Interface)
			fv.Update()
			break

		case zreflect.KindBool:
			b := zbool.ToBoolInd(item.Value.Interface().(bool))
			cv := fview.(*zui.CheckBox)
			v := cv.Value()
			if v != b {
				cv.SetValue(b)
			}

		case zreflect.KindInt, zreflect.KindFloat:
			str := getTextFromNumberishItem(item, f)
			if f.IsStatic() {
				label, _ := fview.(*zui.Label)
				label.SetText(str)
				break
			}
			tv, _ := fview.(*zui.TextView)
			if tv != nil {
				if tv.IsEditing() {
					break
				}
				tv.SetText(str)
			}

		case zreflect.KindString, zreflect.KindFunc:
			str := item.Value.String()
			if f.Flags&flagIsImage != 0 {
				path := ""
				if f.Kind == zreflect.KindString {
					path = str
				}
				if path != "" && strings.Contains(f.ImageFixedPath, "*") {
					path = strings.Replace(f.ImageFixedPath, "*", path, 1)
				} else if path == "" || f.Flags&flagIsFixed != 0 {
					path = f.ImageFixedPath
				}
				iv := fview.(*zui.ImageView)
				iv.SetImage(nil, path, nil)
			} else {
				if f.IsStatic() {
					label, _ := fview.(*zui.Label)
					if f.Flags&flagIsFixed != 0 {
						str = f.Name
					}
					zlog.Assert(label != nil)
					label.SetText(str)
				} else {
					tv, _ := fview.(*zui.TextView)
					if tv != nil {
						if tv.IsEditing() {
							break
						}
						tv.SetText(str)
					}
				}
			}
		}
	}
	// call general one with no id. Needs to be after above loop, so values set
	fh, _ := v.structure.(ActionHandler)
	if fh != nil {
		sview := v.View
		fh.HandleAction(nil, DataChangedAction, &sview)
	}
}

func updateSliceFieldView(view zui.View, item zreflect.Item, f *Field) {
	// zlog.Info("updateSliceFieldView:", view.ObjectName(), item.FieldName, f.Name)
	children := (view.(zui.ContainerType)).GetChildren()
	n := 0
	subViewCount := len(children)
	single := f.Flags&flagIsNamedSelection != 0
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

func FieldViewNew(id string, structure interface{}, labelizeWidth float64) *FieldView {
	v := fieldViewNew(id, true, structure, 12, zgeo.Size{10, 10}, labelizeWidth, nil)
	return v
}

func (v *FieldView) SetStructure(s interface{}) {
	v.structure = s
	// do more here, recursive, and set changed?
}

func (v *FieldView) callActionHandlerFunc(f *Field, action ActionType, fieldValue interface{}, view *zui.View) bool {
	return callActionHandlerFunc(v.structure, f, action, fieldValue, view)
}

func callActionHandlerFunc(structure interface{}, f *Field, action ActionType, fieldValue interface{}, view *zui.View) bool {
	// zlog.Info("callActionHandlerFunc:", f.ID, f.Name, action)
	direct := (action == CreateFieldViewAction || action == SetupFieldAction)
	// zlog.Info("callActionHandlerFunc  get sub:", f.ID, f.Name, action)
	// structure := v.getSubStruct(structID, direct)
	// zlog.Info("callFieldHandler1", action, f.Name, structure != nil, reflect.ValueOf(structure))
	fh, _ := structure.(ActionHandler)
	var result bool
	// zlog.Info("callFieldHandler1", action, f.Name, fh != nil)
	if fh != nil {
		result = fh.HandleAction(f, action, view)
	}

	if view != nil && *view != nil {
		first := true
		n := zui.ViewGetNative(*view)
		for n != nil {
			parent := n.Parent()
			if parent != nil {
				fv, _ := parent.View.(*FieldView)
				// zlog.Info("callFieldHandler parent", action, f.Name, parent.ObjectName(), fv != nil, reflect.ValueOf(parent.View).Type())
				if fv != nil {
					if !first {
						fh2, _ := fv.structure.(ActionHandler)
						if fh2 != nil {
							// zlog.Info("callFieldHandler action2", action, f.Name)
							fh2.HandleAction(nil, action, &parent.View)
						}
					}
					first = false
				}
			}
			n = parent
		}
	}

	if !result {
		if !direct {
			changed := false
			sv := reflect.ValueOf(structure)
			// zlog.Info("\n\nNew struct search for children?:", f.FieldName, sv.Kind(), sv.CanAddr(), v.structure != nil)
			if sv.Kind() == reflect.Ptr || sv.CanAddr() {
				// Here we run thru the possiblly new struct again, and find the item with same id as field
				// s := structure
				// if sv.Kind() != reflect.Ptr {
				// 	s = sv.Addr().Interface()
				// }
				v, found := zreflect.FindFieldWithNameInStruct(f.FieldName, structure, true)
				if found {
					changed = true
					fieldValue = v.Interface()
				}
				// options := zreflect.Options{UnnestAnonymous: true, Recursive: false}
				// items, err := zreflect.ItterateStruct(s, options)
				// // zlog.Info("New struct search for children:", f.FieldName, len(items.Children), err)
				// if err != nil {
				// 	zlog.Fatal(err, "children action")
				// }
				// for _, c := range items.Children {
				// 	// zlog.Info("New struct search for children find:", f.FieldName, c.FieldName)
				// 	if c.FieldName == f.FieldName {
				// 		// zlog.Info("New struct search for children got:", f.FieldName)
				// 		fieldValue = c.Interface
				// 		changed = true
				// 	}
				// }
			}
			if !changed {
				zlog.Info("NOOT!!!", f.Name, action, structure != nil)
				zlog.Fatal(nil, "Not CHANGED!", f.Name)
			}
		}
		aih, _ := fieldValue.(ActionFieldHandler)
		// vvv := reflect.ValueOf(fieldValue)
		// zlog.Info("callActionHandlerFunc bottom:", f.Name, action, result, view, vvv.Kind(), vvv.Type())
		if aih != nil {
			result = aih.HandleFieldAction(f, action, view)
			// zlog.Info("callActionHandlerFunc bottom:", f.Name, action, result, view, aih)
		}
	}
	// zlog.Info("callActionHandlerFunc top done:", f.ID, f.Name, action)
	return result
}

func (v *FieldView) findFieldWithID(id string) *Field {
	for i, f := range v.fields {
		if f.ID == id {
			return &v.fields[i]
		}
	}
	return nil
}

func (fv *FieldView) makeButton(item zreflect.Item, f *Field) *zui.Button {
	// zlog.Info("makeButton:", f.Name, f.Height)
	format := f.Format
	if format == "" {
		format = "%v"
	}
	color := "gray"
	if len(f.Colors) != 0 {
		color = f.Colors[0]
	}
	name := f.Name
	if f.Title != "" {
		name = f.Title
	}
	button := zui.ButtonNew(name, color, zgeo.Size{40, f.Height}, zgeo.Size{}) //ShapeViewNew(ShapeViewTypeRoundRect, s)
	button.SetTextColor(zgeo.ColorBlack)
	button.TextXMargin = 0
	return button
}

func (v *FieldView) makeMenu(item zreflect.Item, f *Field, items zdict.Items) *zui.MenuView {
	menu := zui.MenuViewNew(f.Name+"Menu", items, item.Interface, f.IsStatic())
	menu.SetMaxWidth(f.MaxWidth)

	// zlog.Info("makeMenu2:", f.Name, items.Count(), item.Interface, item.TypeName, item.Kind)

	var view zui.View
	view = menu
	menu.SetSelectedHandler(func(name string, value interface{}) {
		//		zlog.Debug(iface, f.Name)
		v.toDataItem(f, menu, false)
		//		item.Value.Set(reflect.ValueOf(iface))
		if menu.IsStatic {
			val := menu.CurrentValue()
			kind := reflect.ValueOf(val).Kind()
			if kind != reflect.Ptr && kind != reflect.Struct {
				nf := *f
				nf.ActionValue = val
				v.callActionHandlerFunc(&nf, PressedAction, item.Interface, &view)
			}
		} else {
			v.callActionHandlerFunc(f, EditedAction, item.Interface, &view)
		}
	})
	return menu
}

func getTimeString(item zreflect.Item, f *Field) string {
	var str string
	t := item.Interface.(time.Time)
	if t.IsZero() {
		return ""
	}
	format := f.Format
	if format == "" {
		format = "15:04 02-Jan-06"
	}
	// zlog.Info("makeTimeView:", format)
	if format == "nice" {
		str = ztime.GetNice(t, f.Flags&flagHasSeconds != 0)
	} else {
		str = t.Format(format)
	}
	return str
}

func getTextFromNumberishItem(item zreflect.Item, f *Field) string {
	str := ""
	if item.Kind == zreflect.KindTime {
		str = getTimeString(item, f)
	} else if item.Package == "time" && item.TypeName == "Duration" {
		t := ztime.DurSeconds(time.Duration(item.Value.Int()))
		str = ztime.GetSecsAsHMSString(t, f.Flags&flagHasSeconds != 0, 0)
	} else {
		format := f.Format
		if format == "" {
			format = "%v"
		}
		str = fmt.Sprintf(format, item.Value.Interface())
	}
	return str
}

func (v *FieldView) makeText(item zreflect.Item, f *Field) zui.View {
	// zlog.Info("make Text:", item.FieldName, f.Name, v.structure)
	str := getTextFromNumberishItem(item, f)
	if f.IsStatic() {
		label := zui.LabelNew(str)
		j := f.Justify
		if j == zgeo.AlignmentNone {
			j = f.Alignment & (zgeo.Left | zgeo.HorCenter | zgeo.Right)
			if j == zgeo.AlignmentNone {
				j = zgeo.Left
			}
		}
		// label.SetMaxLines(strings.Count(str, "\n") + 1)
		f.SetFont(label, nil)
		label.SetTextAlignment(j)
		if f.Flags&flagToClipboard != 0 {
			label.SetPressedHandler(func() {
				text := label.Text()
				zui.ClipboardSetString(text)
				label.SetText("📋 " + text)
				ztimer.StartIn(0.6, func() {
					label.SetText(text)
				})
			})
		}
		return label
	}
	var style zui.TextViewStyle
	tv := zui.TextViewNew(str, style, f.Columns, f.Rows)
	f.SetFont(tv, nil)
	tv.UpdateSecs = f.UpdateSecs
	tv.SetPlaceholder(f.Placeholder)
	tv.SetChangedHandler(func(view zui.View) {
		v.toDataItem(f, tv, true)
		// zlog.Info("Changed text1:", structure)
		if v.handleUpdate != nil {
			edited := true
			v.handleUpdate(edited)
		}
		// fmt.Printf("Changed text: %p v:%p %+v\n", v.structure, v, v.structure)
		view = zui.View(tv)
		v.callActionHandlerFunc(f, EditedAction, item.Interface, &view)
	})
	tv.SetKeyHandler(func(view zui.View, key zui.KeyboardKey, mods zui.KeyboardModifier) {
		// zlog.Info("keyup!")
	})
	return tv
}

func (v *FieldView) makeCheckbox(item zreflect.Item, f *Field, b zbool.BoolInd) zui.View {
	cv := zui.CheckBoxNew(b)
	cv.SetObjectName(f.ID)
	cv.SetValueHandler(func(_ zui.View) {
		v.toDataItem(f, cv, true)
		view := zui.View(cv)
		v.callActionHandlerFunc(f, EditedAction, item.Interface, &view)
	})
	return cv
}

func (v *FieldView) makeImage(item zreflect.Item, f *Field) zui.View {
	iv := zui.ImageViewNew(nil, "", f.Size)
	iv.SetMinSize(f.Size)
	iv.SetObjectName(f.ID)
	return iv
}

func makeCircledLabelButton(text string, f *Field) *zui.Label {
	w := zui.FontDefaultSize + 14
	label := zui.LabelNew(text)
	font := zui.FontNice(w-8, zui.FontStyleNormal)
	f.SetFont(label, font)
	label.SetMargin(zgeo.RectFromXY2(0, 2, 0, -2))
	label.SetMinWidth(w)
	label.SetCorner(w / 2)
	label.SetTextAlignment(zgeo.Center)
	label.SetBGColor(zgeo.ColorNewGray(0, 0.2))
	return label
}
func (v *FieldView) updateSliceValue(structure interface{}, stack *zui.StackView, vertical bool, f *Field, sendEdited bool) zui.View {
	ct := stack.Parent().View.(zui.ContainerType)
	newStack := v.buildStackFromSlice(structure, vertical, f)
	ct.ReplaceChild(stack, newStack)
	ctp := stack.Parent().Parent().View.(zui.ContainerType)
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
		stack.SetSpacing(0) // f.Spacing)
	}
	key := v.makeNamedSelectionKey(f)
	var selectedIndex int
	single := f.Flags&flagIsNamedSelection != 0
	var fieldView *FieldView
	// zlog.Info("buildStackFromSlice:", vertical, f.ID, val.Len())
	if single {
		selectedIndex, _ = zui.DefaultLocalKeyValueStore.GetInt(key, 0)
		// zlog.Info("buildStackFromSlice:", key, selectedIndex, vertical, f.ID)
		zint.Minimize(&selectedIndex, sliceVal.Len()-1)
		zint.Maximize(&selectedIndex, 0)
		stack.SetMargin(zgeo.RectFromXY2(8, 6, -8, 10))
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
				label := makeCircledLabelButton("–", f)
				fieldView.Add(zgeo.CenterLeft, label)
				index := n
				label.SetPressedHandler(func() {
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
		label := makeCircledLabelButton("+", f)
		label.SetPressedHandler(func() {
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
			bar.Add(zgeo.CenterRight, label)
		} else {
			stack.Add(zgeo.TopLeft, label)
		}
	}
	if single {
		label := makeCircledLabelButton("–", f)
		bar.Add(zgeo.CenterRight, label)
		label.SetPressedHandler(func() {
			val, _ := zreflect.FindFieldWithNameInStruct(f.FieldName, structure, true)
			zslice.RemoveAt(val.Addr().Interface(), selectedIndex)
			// zlog.Info("newlen:", index, val.Len())
			v.updateSliceValue(structure, stack, vertical, f, true)
		})
		label.SetUsable(sliceVal.Len() > 0)
		// zlog.Info("Make Slice thing:", key, selectedIndex, val.Len())

		label = makeCircledLabelButton("⇦", f)
		bar.Add(zgeo.CenterLeft, label)
		label.SetPressedHandler(func() {
			v.changeNamedSelectionIndex(selectedIndex-1, f)
			v.updateSliceValue(structure, stack, vertical, f, false)
		})
		label.SetUsable(selectedIndex > 0)

		str := "0"
		if sliceVal.Len() > 0 {
			str = fmt.Sprintf("%d of %d", selectedIndex+1, sliceVal.Len())
		}
		label = zui.LabelNew(str)
		label.SetColor(zgeo.ColorNewGray(0, 1))
		f.SetFont(label, nil)
		bar.Add(zgeo.CenterLeft, label)

		label = makeCircledLabelButton("⇨", f)
		bar.Add(zgeo.CenterLeft, label)
		label.SetPressedHandler(func() {
			v.changeNamedSelectionIndex(selectedIndex+1, f)
			v.updateSliceValue(structure, stack, vertical, f, false)
		})
		label.SetUsable(selectedIndex < sliceVal.Len()-1)
	}
	return stack
}

// func updateStackFromActionFieldHandlerSlice(view zui.View, item *zreflect.Item, f *Field) bool {
// 	var updated bool
// 	ct, _ := view.(zui.ContainerType)
// 	if ct == nil {
// 		return false
// 	}
// 	views := ct.GetChildren()
// 	for n := 0; n < item.Value.Len(); n++ {
// 		if len(views) <= n {
// 			break
// 		}
// 		h, _ := item.Value.Index(0).Interface().(ActionFieldHandler)
// 		if h != nil {
// 			updated = true
// 			sview := views[n]
// 			ah := item.Value.Index(n).Interface().(ActionFieldHandler)
// 			ah.HandleFieldAction(f, DataChangedAction, &sview)
// 		}
// 	}
// 	return updated
// }

func (v *FieldView) updateSinceTime(label *zui.Label, f *Field) {
	sv := reflect.ValueOf(v.structure)
	// zlog.Info("\n\nNew struct search for children?:", f.FieldName, sv.Kind(), sv.CanAddr(), v.structure != nil)
	zlog.Assert(sv.Kind() == reflect.Ptr || sv.CanAddr())
	// Here we run thru the possiblly new struct again, and find the item with same id as field
	// s := structure
	// if sv.Kind() != reflect.Ptr {
	// 	s = sv.Addr().Interface()
	// }
	val, found := zreflect.FindFieldWithNameInStruct(f.FieldName, v.structure, true)
	if found {
		var str string
		t := val.Interface().(time.Time)
		tooBig := true
		if !t.IsZero() {
			since := time.Since(t)
			str, tooBig = ztime.GetDurationString(since, f.Flags&flagHasSeconds != 0, f.Flags&flagHasMinutes != 0, f.Flags&flagHasHours != 0, f.Columns)
		}
		if tooBig {
			label.SetText("●")
			label.SetColor(zgeo.ColorRed)
		} else {
			label.SetText(str)
			if !v.callActionHandlerFunc(f, DataChangedAction, t, &label.View) {
				col := zgeo.ColorDefaultForeground
				if len(f.Colors) != 0 {
					col = zgeo.ColorFromString(f.Colors[0])
				}
				label.SetColor(col)
			}
		}
	}
}

func (v *FieldView) MakeGroup(f *Field) {
	v.SetMargin(zgeo.RectFromXY2(10, 20, -10, -10))
	v.SetBGColor(zgeo.ColorNewGray(0, 0.1))
	v.SetCorner(8)
	v.SetDrawHandler(func(rect zgeo.Rect, canvas *zui.Canvas, view zui.View) {
		t := zui.TextInfoNew()
		t.Rect = rect
		t.Text = f.Name
		t.Alignment = zgeo.TopLeft
		t.Font = zui.FontNice(zui.FontDefaultSize-3, zui.FontStyleBold)
		t.Color = zgeo.ColorNewGray(0.3, 0.6)
		t.Margin = zgeo.Size{8, 4}
		t.Draw(canvas)
	})
}
func (v *FieldView) buildStack(name string, defaultAlign zgeo.Alignment, cellMargin zgeo.Size, useMinWidth bool, inset float64) {
	zlog.Assert(reflect.ValueOf(v.structure).Kind() == reflect.Ptr, name, v.structure)
	// fmt.Println("buildStack1", name, defaultAlign)
	children := v.getStructItems()
	labelizeWidth := v.labelizeWidth
	if v.parentField != nil && v.labelizeWidth == 0 {
		labelizeWidth = v.parentField.LabelizeWidth
	}
	// zlog.Info("buildStack", name, len(children))
	for j, item := range children {
		exp := zgeo.AlignmentNone
		f := findFieldWithIndex(&v.fields, j)
		if f == nil {
			//			zlog.Error(nil, "no field for index", j)
			continue
		}
		// zlog.Info("   buildStack2", j, f.Name, item)
		// if f.FieldName == "CPU" {
		// 	zlog.Info("   buildStack1.2", j, item.Value.Len())
		// }

		var view zui.View
		if f.Flags&flagIsButton != 0 {
			view = v.makeButton(item, f)
		} else {
			v.callActionHandlerFunc(f, CreateFieldViewAction, item.Interface, &view) // this sees if actual ITEM is a field handler
			// if called {
			// 	zlog.Info("CALLED:", f.Name, view)
			// }
		}
		if view != nil {
		} else if f.LocalEnum != "" {
			ei := findLocalField(&children, f.LocalEnum)
			if !zlog.ErrorIf(ei == nil, f.Name, f.LocalEnum) {
				getter, _ := ei.Interface.(zdict.ItemsGetter)
				if !zlog.ErrorIf(getter == nil, "field isn't enum, not ItemGetter type", f.Name, f.LocalEnum) {
					enum := getter.GetItems()
					// zlog.Info("make local enum:", f.Name, f.LocalEnum, enum, ei)
					// 	continue
					// }
					// zlog.Info("make local enum:", f.Name, f.LocalEnum, i, MenuItemsLength(enum))
					menu := v.makeMenu(item, f, enum)
					if menu == nil {
						zlog.Error(nil, "no local enum for", f.LocalEnum)
						continue
					}
					view = menu
					menu.SelectWithValue(item.Interface)
				}
			}
		} else if f.Enum != "" {
			// fmt.Printf("make enum: %s %v\n", f.Name, item)
			enum, _ := fieldEnums[f.Enum]
			zlog.Assert(enum != nil)
			view = v.makeMenu(item, f, enum)
			exp = zgeo.AlignmentNone
		} else {
			switch f.Kind {
			case zreflect.KindStruct:
				_, got := item.Interface.(zui.UIStringer)
				// zlog.Info("make stringer?:", f.Name, got)
				if got && f.IsStatic() {
					view = v.makeText(item, f)
				} else {
					exp = zgeo.HorExpand
					// zlog.Info("struct make field view:", f.Name, f.Kind, exp)
					childStruct := item.Address
					vertical := true
					fieldView := fieldViewNew(f.ID, vertical, childStruct, 10, zgeo.Size{}, labelizeWidth, v)
					fieldView.parentField = f
					if f.IsGroup {
						fieldView.MakeGroup(f)
					}
					view = fieldView
					fieldView.buildStack(f.ID, zgeo.TopLeft, zgeo.Size{}, true, 5)
				}

			case zreflect.KindBool:
				b := zbool.ToBoolInd(item.Value.Interface().(bool))
				exp = zgeo.AlignmentNone
				view = v.makeCheckbox(item, f, b)

			case zreflect.KindInt:
				if item.TypeName == "BoolInd" {
					exp = zgeo.HorShrink
					view = v.makeCheckbox(item, f, zbool.BoolInd(item.Value.Int()))
				} else {
					view = v.makeText(item, f)
				}

			case zreflect.KindFloat:
				view = v.makeText(item, f)

			case zreflect.KindString:
				if f.Flags&flagIsImage != 0 {
					view = v.makeImage(item, f)
				} else {
					if (f.MaxWidth != f.MinWidth || f.MaxWidth != 0) && f.Flags&flagIsButton == 0 {
						exp = zgeo.HorExpand
					}
					// zlog.Info("Make Text:", f.Name, f.MaxWidth, f.MinWidth, f.Size, exp, defaultAlign)
					view = v.makeText(item, f)
				}

			case zreflect.KindSlice:
				getter, _ := item.Interface.(zdict.ItemsGetter)
				if getter != nil {
					menu := v.makeMenu(item, f, getter.GetItems())
					view = menu
					break
				}
				//				zlog.Info("Make slice:", item.Address, item.Value.CanAddr(), item.Value.CanSet(), reflect.ValueOf(item.Value.Interface()).CanAddr())
				exp = zgeo.Expand
				view = v.buildStackFromSlice(v.structure, v.Vertical, f)
				break
				// 	// view = createStackFromActionFieldHandlerSlice(&item, f)
				// }
				// view = v.makeText(item, f)
				break

			case zreflect.KindTime:
				view = v.makeText(item, f)
				if f.Flags&flagIsDuration != 0 {
					timer := ztimer.RepeatNow(1, func() bool {
						v.updateSinceTime(view.(*zui.Label), f)
						return true
					})
					zui.ViewGetNative(view).AddStopper(timer)
				}

			default:
				panic(fmt.Sprintln("buildStack bad type:", f.Name, f.Kind))
			}
		}
		pt, _ := view.(zui.Pressable)
		if pt != nil {
			ph := pt.PressedHandler()
			nowItem := item // store item in nowItem so closures below uses right item
			pt.SetPressedHandler(func() {
				if !v.callActionHandlerFunc(f, PressedAction, nowItem.Interface, &view) && ph != nil {
					ph()
				}
			})
			lph := pt.LongPressedHandler()
			pt.SetLongPressedHandler(func() {
				// zlog.Info("Field.LPH:", f.ID)
				if !v.callActionHandlerFunc(f, LongPressedAction, nowItem.Interface, &view) && lph != nil {
					lph()
				}
			})

		}
		var tipField, tip string
		if zstr.HasPrefix(f.Tooltip, ".", &tipField) {
			for _, ei := range children {
				if ei.FieldName == tipField {
					tip = fmt.Sprint(ei.Interface)
					break
				}
			}
		} else if f.Tooltip != "" {
			tip = f.Tooltip
		}
		if tip != "" {
			zui.ViewGetNative(view).SetToolTip(tip)
		}
		if !f.Shadow.Delta.IsNull() {
			nv := zui.ViewGetNative(view)
			nv.SetDropShadow(f.Shadow)
		}
		view.SetObjectName(f.ID)
		if len(f.Colors) != 0 {
			view.SetColor(zgeo.ColorFromString(f.Colors[0]))
		}
		cell := &zui.ContainerViewCell{}
		if labelizeWidth != 0 {
			var lstack *zui.StackView
			title := f.Name
			if f.Flags&flagNoTitle != 0 {
				title = ""
			}
			_, lstack, cell = zui.Labelize(view, title, labelizeWidth)
			v.AddView(lstack, zgeo.HorExpand|zgeo.Left|zgeo.Top)
		}
		cell.Margin = cellMargin
		def := defaultAlign
		all := zgeo.Left | zgeo.HorCenter | zgeo.Right
		if f.Alignment&all != 0 {
			def &= ^all
		}
		cell.Alignment = def | exp | f.Alignment
		// zlog.Info("field align2:", f.Alignment, f.Name, def, f.Flags&flagIsButton, exp, cell.Alignment, int(cell.Alignment))
		if useMinWidth {
			cell.MinSize.W = f.MinWidth
			// zlog.Info("Cell Width:", f.Name, cell.MinSize.W)
		}
		cell.MaxSize.W = f.MaxWidth
		if cell.MinSize.W != 0 && (j == 0 || j == len(children)-1) {
			cell.MinSize.W += inset
		}
		if f.Flags&flagExpandFromMinSize != 0 {
			cell.ExpandFromMinSize = true
		}
		// zlog.Info("Add Field Item:", cell.View.ObjectName(), cell.Alignment, f.MinWidth, cell.MinSize.W, cell.MaxSize)
		if labelizeWidth == 0 {
			cell.View = view
			v.AddCell(*cell, -1)
		}
	}
}

func (v *FieldView) toDataItem(f *Field, view zui.View, showError bool) error {
	var err error

	if f.Flags&flagIsStatic != 0 {
		return nil
	}
	children := v.getStructItems()
	// zlog.Info("fieldViewToDataItem before:", f.Name, f.Index, len(children), "s:", structure)
	item := children[f.Index]
	if (f.Enum != "" || f.LocalEnum != "") && !f.IsStatic() {
		mv, _ := view.(*zui.MenuView)
		if mv != nil {
			iface := mv.CurrentValue()
			vo := reflect.ValueOf(iface)
			// zlog.Debug(iface, f.Name, iface == nil)
			if iface == nil {
				vo = reflect.Zero(item.Value.Type())
			}
			item.Value.Set(vo)
		}
		return nil
	}

	switch f.Kind {
	case zreflect.KindBool:
		bv, _ := view.(*zui.CheckBox)
		if bv == nil {
			panic("Should be switch")
		}
		b, _ := item.Address.(*bool)
		if b != nil {
			*b = bv.Value().BoolValue()
		}
		bi, _ := item.Address.(*zbool.BoolInd)
		if bi != nil {
			*bi = bv.Value()
		}

	case zreflect.KindInt:
		if !f.IsStatic() {
			if item.TypeName == "BoolInd" {
				bv, _ := view.(*zui.CheckBox)
				*item.Address.(*bool) = bv.Value().BoolValue()
			} else {
				tv, _ := view.(*zui.TextView)
				str := tv.Text()
				if item.Package == "time" && item.TypeName == "Duration" {
					var secs float64
					secs, err = ztime.GetSecsFromHMSString(str, f.Flags&flagHasHours != 0, f.Flags&flagHasMinutes != 0, f.Flags&flagHasSeconds != 0)
					if err != nil {
						break
					}
					d := item.Address.(*time.Duration)
					if d != nil {
						*d = ztime.SecondsDur(secs)
					}
					return nil
				}
				var i64 int64
				i64, err = strconv.ParseInt(str, 10, 64)
				if err != nil {
					break
				}
				zint.SetAny(item.Address, i64)
			}
		}

	case zreflect.KindFloat:
		if f.Flags&flagIsStatic == 0 {
			tv, _ := view.(*zui.TextView)
			var f64 float64
			f64, err = strconv.ParseFloat(tv.Text(), 64)
			if err != nil {
				break
			}
			zfloat.SetAny(item.Address, f64)
		}

	case zreflect.KindTime:
		break

	case zreflect.KindString:
		if !f.IsStatic() && f.Flags&flagIsImage == 0 {
			tv, _ := view.(*zui.TextView)
			if tv == nil {
				zlog.Fatal(nil, "Copy Back string not TV:", f.Name)
			}
			text := tv.Text()
			str := item.Address.(*string)
			*str = text
		}

	case zreflect.KindFunc:
		break

	default:
		panic(fmt.Sprint("bad type: ", f.Kind))
	}

	if showError && err != nil {
		zui.AlertShowError("", err)
	}
	return err
}

func ParentFieldView(view zui.View) *FieldView {
	for _, nv := range zui.ViewGetNative(view).AllParents() {
		fv, _ := nv.View.(*FieldView)
		if fv != nil {
			return fv
		}
	}
	return nil
}

func (fv *FieldView) getStructItems() []zreflect.Item {
	k := reflect.ValueOf(fv.structure).Kind()
	// zlog.Info("getStructItems", direct, k, sub)
	zlog.Assert(k == reflect.Ptr, "not pointer", k)
	options := zreflect.Options{UnnestAnonymous: true, Recursive: false}
	rootItems, err := zreflect.ItterateStruct(fv.structure, options)
	if err != nil {
		panic(err)
	}
	// for _, c := range rootItems.Children {
	// 	if c.FieldName == "CPU" {
	// 		zlog.Info("CPU COunt:", c.Value.Len())
	// 	}
	// }
	// zlog.Info("getStructItems DONE", k)
	// zlog.Info("Get Struct Items sub:", len(rootItems.Children))
	return rootItems.Children
}
