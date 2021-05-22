// +build zui

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
	"github.com/torlangballe/zutil/zstr"
	"github.com/torlangballe/zutil/ztime"
	"github.com/torlangballe/zutil/ztimer"
	"github.com/torlangballe/zutil/zwords"
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
			v, _ := s.FindViewWithName(name, false)
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
		if f.LocalShow != "" {
			eItem := findLocalFieldWithID(&children, f.LocalShow)
			show, got := eItem.Interface.(bool)
			if got {
				parent := zui.ViewGetNative(fview).Parent()
				if parent != nil && parent != &v.NativeView { // it has a holding parent that is what actually shoule be shown/unshown
					parent.Show(show)
				} else {
					fview.Show(show)
				}
			}
		}
		if f.LocalEnable != "" {
			eItem := findLocalFieldWithID(&children, f.LocalEnable)
			enabled, got := eItem.Interface.(bool)
			if got {
				parent := zui.ViewGetNative(fview).Parent()
				if parent != nil && parent != &v.NativeView { // it has a holding parent that is what actually shoule be en/dis-abled
					parent.SetUsable(enabled)
				} else {
					fview.SetUsable(enabled)
				}
			}
		}
		if f.Flags&flagIsButton != 0 {
			enabled, is := item.Interface.(bool)
			if is {
				fview.SetUsable(enabled)
			}
			continue
		}
		called := v.callActionHandlerFunc(f, DataChangedAction, item.Interface, &fview)
		if called {
			continue
		}
		menuType, _ := fview.(zui.MenuType)
		if menuType != nil && ((f.Enum != "" && f.Kind != zreflect.KindSlice) || f.LocalEnum != "") {
			var enum zdict.Items
			// zlog.Info("Update FV: Menu:", f.Name, f.Enum, f.LocalEnum)
			if f.Enum != "" {
				enum, _ = fieldEnums[f.Enum]
				// zlog.Info("UpdateStack Enum:", f.Name)
				// zdict.DumpNamedValues(enum)
			} else {
				ei := findLocalFieldWithID(&children, f.LocalEnum)
				zlog.Assert(ei != nil, f.Name, f.LocalEnum)
				enum = ei.Interface.(zdict.ItemsGetter).GetItems()
			}
			// zlog.Assert(enum != nil, f.Name, f.LocalEnum, f.Enum)
			menuType.UpdateItems(enum, []interface{}{item.Interface})
			continue
		}
		if menuType == nil && f.Kind == zreflect.KindSlice {
			// val, found := zreflect.FindFieldWithNameInStruct(f.FieldName, v.structure, true)
			// fmt.Printf("updateSliceFieldView: %s %p %p %v %p\n", v.id, item.Interface, val.Interface(), found, view)
			updateSliceFieldView(fview, item, f)
		}
		updateItemLocalToolTip(f, children, fview)

		switch f.Kind {
		case zreflect.KindSlice:
			getter, _ := item.Interface.(zdict.ItemsGetter)
			if getter != nil {
				items := getter.GetItems()
				mt := fview.(zui.MenuType)
				// zlog.Info("fv update slice:", f.Name, len(items), mt != nil, reflect.ValueOf(fview).Type())
				if mt != nil {
					// assert menu is static...
					mt.UpdateItems(items, nil)
				}
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
			_, got := item.Interface.(UIStringer)
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
			_, got := item.Interface.(zbool.BitsetItemsOwner)
			if got {
				updateFlagStack(item, f, fview)
			}

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
				io := fview.(zui.ImageOwner)
				io.SetImage(nil, path, nil)
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

func (fv *FieldView) makeButton(item zreflect.Item, f *Field) *zui.ImageButtonView {
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
	button := zui.ImageButtonViewNew(name, color, zgeo.Size{40, f.Height}, zgeo.Size{}) //ShapeViewNew(ShapeViewTypeRoundRect, s)
	button.SetTextColor(zgeo.ColorBlack)
	button.TextXMargin = 0
	return button
}

func (v *FieldView) makeMenu(item zreflect.Item, f *Field, items zdict.Items) zui.View {
	var view zui.View

	if f.IsStatic() {
		multi := false
		// zlog.Info("FV Menu Make static:", f.ID, f.Format, f.Name)
		vals := []interface{}{item.Interface}
		isImage := (f.ImageFixedPath != "")
		shape := zui.ShapeViewTypeRoundRect
		if isImage {
			shape = zui.ShapeViewTypeNone
		}
		var mItems []zui.MenuedItem
		for i := range items {
			var m zui.MenuedItem
			for j := range vals {
				if reflect.DeepEqual(items[i], vals[j]) {
					m.Selected = true
					break
				}
			}
			if f.Flags&flagIsActions != 0 {
				m.IsAction = true
			}
			m.Item = items[i]
			mItems = append(mItems, m)
		}
		menu := zui.MenuedShapeViewNew(shape, zgeo.Size{20, 20}, f.ID, mItems, f.IsStatic(), multi)
		if isImage {
			menu.SetImage(nil, f.ImageFixedPath, nil)
			menu.ImageAlign = zgeo.Center | zgeo.Proportional
			// zlog.Info("FV Menued:", f.ID, f.Size)
			menu.ImageMaxSize = f.Size
		} else {
			menu.SetPillStyle()
			if len(f.Colors) != 0 {
				menu.SetColor(zgeo.ColorFromString(f.Colors[0]))
			}
		}
		view = menu
		// zlog.Info("Make Menu Format", f.Name, f.Format)
		if f.Format != "" {
			if f.Format == "-" {
				menu.GetTitle = func(icount int) string {
					return ""
				}
			} else if f.Format == "%d" {
				menu.GetTitle = func(icount int) string {
					// zlog.Info("fv menu gettitle2:", f.FieldName, f.Format, icount)
					return strconv.Itoa(icount)
				}
			} else {
				menu.GetTitle = func(icount int) string {
					// zlog.Info("fv menu gettitle:", f.FieldName, f.Format, icount)
					return zwords.PluralWord(f.Format, float64(icount), "", "", 0)
				}
			}
		}
		menu.SetSelectedHandler(func() {
			v.toDataItem(f, menu, false)
			if menu.IsStatic {
				sel := menu.SelectedItem()
				kind := reflect.ValueOf(sel.Value).Kind()
				// zlog.Info("action pressed", kind, sel.Name, "val:", sel.Value)
				if kind != reflect.Ptr && kind != reflect.Struct {
					nf := *f
					nf.ActionValue = sel.Value
					v.callActionHandlerFunc(&nf, PressedAction, item.Interface, &view)
				}
			} else {
				v.callActionHandlerFunc(f, EditedAction, item.Interface, &view)
			}
		})
	} else {
		menu := zui.MenuViewNew(f.Name+"Menu", items, item.Interface)
		menu.SetMaxWidth(f.MaxWidth)
		view = menu
		menu.SetSelectedHandler(func() {
			v.toDataItem(f, menu, false)
			v.callActionHandlerFunc(f, EditedAction, item.Interface, &view)
		})
	}
	return view
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
		label.SetMaxLines(f.Rows)
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
	cols := f.Columns
	if cols == 0 {
		cols = 20
	}
	tv := zui.TextViewNew(str, style, cols, f.Rows)
	f.SetFont(tv, nil)
	tv.UpdateSecs = f.UpdateSecs
	tv.SetPlaceholder(f.Placeholder)
	tv.SetChangedHandler(func() {
		v.toDataItem(f, tv, true)
		// zlog.Info("Changed text1:", structure)
		if v.handleUpdate != nil {
			edited := true
			v.handleUpdate(edited)
		}
		// fmt.Printf("Changed text: %p v:%p %+v\n", v.structure, v, v.structure)
		view := zui.View(tv)
		v.callActionHandlerFunc(f, EditedAction, item.Interface, &view)
	})
	tv.SetKeyHandler(func(key zui.KeyboardKey, mods zui.KeyboardModifier) {
		// zlog.Info("keyup!")
	})
	// zlog.Info("FV makeText:", f.FieldName, tv.MinWidth, tv.Columns)
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
	iv.OpaqueDraw = (f.Flags&flagIsOpaque != 0)
	return iv
}

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
			str, tooBig = ztime.GetDurationString(since, f.Flags&flagHasSeconds != 0, f.Flags&flagHasMinutes != 0, f.Flags&flagHasHours != 0, f.FractionDecimals)
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

func makeFlagStack(flags zreflect.Item, f *Field) zui.View {
	stack := zui.StackViewHor("flags")
	stack.SetMinSize(zgeo.Size{20, 20})
	stack.SetSpacing(2)
	return stack
}

func updateFlagStack(flags zreflect.Item, f *Field, view zui.View) {
	stack := view.(*zui.StackView)
	// zlog.Info("zfields.updateFlagStack", Name(f))
	bso := flags.Interface.(zbool.BitsetItemsOwner)
	bitset := bso.GetBitsetItems()
	n := flags.Value.Int()
	for _, bs := range bitset {
		name := bs.Name
		vf, _ := stack.FindViewWithName(name, false)
		if n&bs.Mask != 0 {
			if vf == nil {
				path := "images/" + f.ID + "/" + name + ".png"
				// zlog.Info("flag image:", name, path)
				iv := zui.ImageViewNew(nil, path, zgeo.Size{16, 16})
				iv.SetObjectName(name) // very important as we above find it in stack
				iv.SetMinSize(zgeo.Size{16, 16})
				stack.Add(iv, zgeo.Center)
				if stack.Presented {
					stack.ArrangeChildren(nil)
				}
				title := bs.Title
				iv.SetToolTip(title)
			}
		} else {
			if vf != nil {
				stack.RemoveNamedChild(name, false)
				stack.ArrangeChildren(nil)
			}
		}
	}
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
			ei := findLocalFieldWithID(&children, f.LocalEnum)
			if !zlog.ErrorIf(ei == nil, f.Name, f.LocalEnum) {
				getter, _ := ei.Interface.(zdict.ItemsGetter)
				if !zlog.ErrorIf(getter == nil, "field isn't enum, not ItemGetter type", f.Name, f.LocalEnum) {
					enum := getter.GetItems()
					// zlog.Info("make local enum:", f.Name, f.LocalEnum, enum, ei)
					// 	continue
					// }
					//					zlog.Info("make local enum:", f.Name, f.LocalEnum)
					menu := v.makeMenu(item, f, enum)
					if menu == nil {
						zlog.Error(nil, "no local enum for", f.LocalEnum)
						continue
					}
					view = menu
					// mt := view.(zui.MenuType)
					//!!					mt.SelectWithValue(item.Interface)
				}
			}
		} else if f.Enum != "" {
			// fmt.Println("make enum:", f.Name)
			enum, _ := fieldEnums[f.Enum]
			zlog.Assert(enum != nil, f.Enum)
			view = v.makeMenu(item, f, enum)
			exp = zgeo.AlignmentNone
		} else {
			switch f.Kind {
			case zreflect.KindStruct:
				_, got := item.Interface.(UIStringer)
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
					_, got := item.Interface.(zbool.BitsetItemsOwner)
					if got {
						view = makeFlagStack(item, f)
						break
					}
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
				//				zlog.Info("Make slice:", v.ObjectName(), f.FieldName, , labelizeWidth)
				if f.Alignment != zgeo.AlignmentNone {
					exp = zgeo.Expand
				} else {
					exp = zgeo.AlignmentNone
				}
				vert := v.Vertical
				if labelizeWidth != 0 {
					vert = false
				}
				view = v.buildStackFromSlice(v.structure, vert, f)
				break
				// 	// view = createStackFromActionFieldHandlerSlice(&item, f)
				// }
				// view = v.makeText(item, f)
				break

			case zreflect.KindTime:
				view = v.makeText(item, f)
				if f.Flags&flagIsDuration != 0 && f.IsStatic() {
					label := view.(*zui.Label)
					label.Columns = f.Columns
					timer := ztimer.RepeatNow(1, func() bool {
						label := view.(*zui.Label)
						v.updateSinceTime(label, f)
						return true
					})
					zui.ViewGetNative(view).AddStopper(timer.Stop)
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
		updateItemLocalToolTip(f, children, view)
		if !f.Shadow.Delta.IsNull() {
			nv := zui.ViewGetNative(view)
			nv.SetDropShadow(f.Shadow)
		}
		view.SetObjectName(f.ID)
		if len(f.Colors) != 0 {
			view.SetColor(zgeo.ColorFromString(f.Colors[0]))
		}
		v.callActionHandlerFunc(f, CreatedViewAction, item.Interface, &view)
		cell := &zui.ContainerViewCell{}
		if labelizeWidth != 0 {
			var lstack *zui.StackView
			title := f.Name
			if f.Flags&flagNoTitle != 0 {
				title = ""
			}
			_, lstack, cell = zui.Labelize(view, title, labelizeWidth)
			v.AddView(lstack, zgeo.HorExpand|zgeo.Left|zgeo.Top)
			// zlog.Info("LABS:", v.ObjectName(), view.ObjectName(), defaultAlign, exp, f.Alignment)
		}
		cell.Margin = cellMargin
		def := defaultAlign
		all := zgeo.Left | zgeo.HorCenter | zgeo.Right
		if f.Alignment&all != 0 {
			def &= ^all
		}
		cell.Alignment = def | exp | f.Alignment
		if useMinWidth {
			cell.MinSize.W = f.MinWidth
		}
		cell.MaxSize.W = f.MaxWidth
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

func updateItemLocalToolTip(f *Field, children []zreflect.Item, view zui.View) {
	var tipField, tip string
	found := false
	if zstr.HasPrefix(f.Tooltip, ".", &tipField) {
		for _, ei := range children {
			// zlog.Info("updateItemLocalToolTip:", fieldNameToID(ei.FieldName), tipField)
			if fieldNameToID(ei.FieldName) == tipField {
				tip = fmt.Sprint(ei.Interface)
				found = true
				break
			}
		}
		if !found { // can't use tip == "" to check, since field might just be empty
			zlog.Error(nil, "updateItemLocalToolTip: no local field for tip", f.Name, tipField)
		}
	} else if f.Tooltip != "" {
		tip = f.Tooltip
	}
	if tip != "" {
		zui.ViewGetNative(view).SetToolTip(tip)
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
			panic("Should be checkbox")
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
	// zlog.Info("Get Struct Items sub:", len(rootItems.Children))
	return rootItems.Children
}
