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

type fieldType int

type ActionType string

const (
	DataChangedAction ActionType = "changed"
	EditedAction      ActionType = "edited"
	SetupAction       ActionType = "setup"
	PressedAction     ActionType = "pressed"
	LongPressedAction ActionType = "longpressed"
	CreateAction      ActionType = "create"
)

const (
	flagIsStatic = 1 << iota
	flagHasSeconds
	flagHasMinutes
	flagHasHours
	flagHasDays
	flagHasMonths
	flagHasYears
	flagIsImage
	flagImageIsFixed
	flagIsButton
	flagHasHeaderImage
	flagNoTitle
	flagToClipboard
	flagIsMenuedGroup
	flagIsTabGroup
	flagIsStringer
)
const (
	flagTimeFlags = flagHasSeconds | flagHasMinutes | flagHasHours
	flagDateFlags = flagHasDays | flagHasMonths | flagHasYears
)

type Field struct {
	Index         int
	ID            string
	Name          string
	FieldName     string
	Title         string // name of item in row, and header if no title
	Width         float64
	MaxWidth      float64
	MinWidth      float64
	Kind          zreflect.TypeKind
	Alignment     zgeo.Alignment
	Justify       zgeo.Alignment
	Format        string
	Colors        []string
	FixedPath     string
	Password      bool
	Height        float64
	Weight        float64
	Enum          string
	LocalEnum     string
	Size          zgeo.Size
	Flags         int
	DefaultWeight float64
	Tooltip       string
	UpdateSecs    float64
	LabelizeWidth float64
	LocalEnable   string
	FontSize      float64
	FontName      string
	FontStyle     zui.FontStyle
	Spacing       float64
}

type ActionHandler interface {
	HandleAction(id string, structID string, f *Field, action ActionType, view *zui.View) bool
}

type ActionFieldHandler interface {
	HandleFieldAction(f *Field, action ActionType, view *zui.View) bool
}

type fieldOwner struct {
	fields        []Field
	structure     interface{} // structure of ALL, not just a row
	id            string
	handleUpdate  func(edited bool, structID string)
	labelizeWidth float64
	getSubStruct  func(structID string) interface{}
}

func (fo *fieldOwner) callActionHandlerFunc(structID string, f *Field, action ActionType, fieldValue interface{}, view *zui.View) bool {
	var structure = fo.structure
	// zlog.Info("callActionHandlerFunc:", f.ID, f.Name, action, structID)
	if structID != "" {
		structure = fo.getSubStruct(structID)
	}
	// zlog.Info("callFieldHandler1", action, f.Name, structure != nil, reflect.ValueOf(structure))
	fh, _ := structure.(ActionHandler)
	// zlog.Info("callFieldHandler1", action, f.Name, fh)
	var result bool
	if fh != nil {
		result = fh.HandleAction(f.ID, structID, f, action, view)
	}

	if view != nil && *view != nil {
		first := true
		n := zui.ViewGetNative(*view)
		for n != nil {
			parent := n.Parent()
			if parent != nil {
				fv, _ := parent.View.(*FieldView)
				if fv != nil {
					if !first {
						fh2, _ := fv.fieldOwner.structure.(ActionHandler)
						if fh2 != nil {
							id2 := zstr.FirstToLowerWithAcronyms(parent.View.ObjectName())
							f2 := fv.fieldOwner.findFieldWithID(id2)
							fh2.HandleAction(id2, structID, f2, action, &parent.View)
						}
					}
					first = false
				}
			}
			n = parent
		}
	}
	if !result {
		unnestAnon := true
		recursive := false
		changed := false
		sv := reflect.ValueOf(structure)
		// zlog.Info("\n\nNew struct search for children?:", f.FieldName, sv.Kind(), sv.CanAddr(), fo.structure != nil)
		if sv.Kind() == reflect.Ptr || sv.CanAddr() {
			s := structure
			if sv.Kind() != reflect.Ptr {
				s = sv.Addr().Interface()
			}
			items, err := zreflect.ItterateStruct(s, unnestAnon, recursive)
			// zlog.Info("New struct search for children:", f.FieldName, len(items.Children), err)
			if err != nil {
				zlog.Fatal(err, "children action")
			}
			for _, c := range items.Children {
				if c.FieldName == f.FieldName {
					fieldValue = c.Interface
					changed = true
				}
			}
		}
		if !changed {
			zlog.Info("NOOT!!!", f.Name, action, structure != nil)
			zlog.Fatal(nil, "Not CHANGED!", f.Name)
		}
		aih, _ := fieldValue.(ActionFieldHandler)
		// vvv := reflect.ValueOf(fieldValue)
		// zlog.Info("callActionHandlerFunc bottom:", f.Name, action, result, view, vvv.Kind(), vvv.Type())
		if aih != nil {
			result = aih.HandleFieldAction(f, action, view)
			// zlog.Info("callActionHandlerFunc bottom:", f.Name, action, result, view, aih)
		} else {
			//			fvv := reflect.ValueOf(fieldValue)
		}
	}
	return result
}

func (fo *fieldOwner) findFieldWithID(id string) *Field {
	for i, f := range fo.fields {
		if f.ID == id {
			return &fo.fields[i]
		}
	}
	return nil
}

func (f Field) IsStatic() bool {
	return f.Flags&flagIsStatic != 0
}

func (fo *fieldOwner) makeButton(item zreflect.Item, f *Field, structID string) *zui.Button {
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
	button.SetColor(zgeo.ColorWhite)
	button.TextXMargin = 0
	return button
}

func (fo *fieldOwner) makeMenu(item zreflect.Item, f *Field, structID string, items zdict.NamedValues) *zui.MenuView {
	menu := zui.MenuViewNew(f.Name+"Menu", items, item.Interface, f.IsStatic())
	menu.SetMaxWidth(f.MaxWidth)

	// zlog.Info("makeMenu2:", f.Name, items.Count(), item.Interface, item.TypeName, item.Kind)

	menu.ChangedHandler(func(id, name string, value interface{}) {
		//		zlog.Debug(iface, f.Name)
		fo.fieldViewToDataItem(structID, f, menu, false)
		//		item.Value.Set(reflect.ValueOf(iface))
		fo.callActionHandlerFunc(structID, f, EditedAction, item.Interface, nil)
	})
	return menu
}

func getTimeString(item zreflect.Item, f *Field) string {
	var str string
	t := item.Interface.(time.Time)
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

func setFontIfSpecifiedInField(f *Field, view zui.View) {
	if f.FontName != "" || f.FontSize != 0 || f.FontStyle != zui.FontStyleNormal {
		to := view.(zui.TextLayoutOwner)
		size := f.FontSize
		if size == 0 {
			size = zui.FontDefaultSize
		}
		var font *zui.Font
		if f.FontName == "" {
			font = zui.FontNice(size, f.FontStyle)
		} else {
			font = zui.FontNew(f.FontName, size, f.FontStyle)
		}
		to.SetFont(font)
	}
}

func (fo *fieldOwner) makeText(item zreflect.Item, f *Field, structID string) zui.View {
	// zlog.Info("make Text:", item.FieldName, f.Name, fo.structure)
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

		setFontIfSpecifiedInField(f, label)
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
	tv := zui.TextViewNew(str, style)
	setFontIfSpecifiedInField(f, tv)
	tv.UpdateSecs = f.UpdateSecs
	tv.ChangedHandler(func(view zui.View) {
		fo.fieldViewToDataItem(structID, f, tv, true)
		// zlog.Info("Changed text1:", structure)
		if fo.handleUpdate != nil {
			edited := true
			fo.handleUpdate(edited, structID)
		}
		// zlog.Info("Changed text:", structure)
		view = zui.View(tv)
		fo.callActionHandlerFunc(structID, f, EditedAction, item.Interface, &view)
	})
	tv.KeyHandler(func(view zui.View, key zui.KeyboardKey, mods zui.KeyboardModifier) {
		zlog.Info("keyup!")
	})
	return tv
}

func (fo *fieldOwner) makeCheckbox(item zreflect.Item, f *Field, structID string, b zbool.BoolInd) zui.View {
	cv := zui.CheckBoxNew(b)
	cv.SetObjectName(f.ID)
	cv.ValueHandler(func(v zui.View) {
		fo.fieldViewToDataItem(structID, f, cv, true)
		view := zui.View(cv)
		fo.callActionHandlerFunc(structID, f, EditedAction, item.Interface, &view)
	})
	return cv
}

func (fo *fieldOwner) makeImage(item zreflect.Item, f *Field, structID string) zui.View {
	iv := zui.ImageViewNew("", f.Size)
	iv.SetMinSize(f.Size)
	iv.SetObjectName(f.ID)
	return iv
}

func findFieldWithIndex(fields *[]Field, index int) *Field {
	for i, f := range *fields {
		if f.Index == index {
			return &(*fields)[i]
		}
	}
	return nil
}

func (fo *fieldOwner) getStructItems(structID string, recursive bool) []zreflect.Item {
	sub := fo.getSubStruct((structID))
	// zlog.Info("getStructItems", structID, sub)
	k := reflect.ValueOf(sub).Kind()
	zlog.Assert(k == reflect.Ptr, "not pointer", k, sub)
	unnestAnon := true
	rootItems, err := zreflect.ItterateStruct(sub, unnestAnon, recursive)
	if err != nil {
		panic(err)
	}
	// zlog.Info("Get Struct Items sub:", len(rootItems.Children))
	return rootItems.Children
}

func (fo *fieldOwner) updateStack(stack *zui.StackView, structID string) {
	children := fo.getStructItems(structID, true)
	// zlog.Info("fields updateStack", stack.ObjectName(), len(children))

	for i, item := range children {
		f := findFieldWithIndex(&fo.fields, i)
		if f == nil {
			continue
		}
		fview := stack.FindViewWithName(f.ID, true)
		if fview == nil {
			continue
		}
		view := *fview
		called := fo.callActionHandlerFunc(structID, f, DataChangedAction, item.Interface, &view)
		if called {
			continue
		}
		menu, _ := view.(*zui.MenuView)
		if f.Enum != "" && f.Kind != zreflect.KindSlice || f.LocalEnum != "" {
			var enum zdict.NamedValues
			if f.Enum != "" {
				enum, _ = fieldEnums[f.Enum]
				// zlog.Info("UpdateStack Enum:", f.Name)
				// zdict.DumpNamedValues(enum)
			} else {
				ei := findLocalField(&children, f.LocalEnum)
				zlog.Assert(ei != nil, f.Name, f.LocalEnum)
				enum, _ = ei.Interface.(zdict.NamedValues)
			}
			zlog.Assert(enum != nil, f.Name, f.LocalEnum, f.Enum)
			menu.UpdateValues(enum)
			menu.SetWithIdOrValue(item.Interface)
			continue
		}
		if f.LocalEnable != "" {
			eItem := findLocalField(&children, f.LocalEnable)
			e, got := eItem.Interface.(bool)
			// zlog.Info("updateStack localEnable:", f.Name, f.LocalEnable, e, got)
			if got {
				parent := zui.ViewGetNative(view).Parent()
				//				if parent != nil && parent != stack(strings.HasPrefix(parent.ObjectName(), "$labelize.") || strings.HasPrefix(parent.ObjectName(), "$labledCheckBoxStack.")) {
				if parent != nil && parent != &stack.NativeView {
					parent.SetUsable(e)
				} else {
					view.SetUsable(e)
				}
			}
		}
		if f.Flags&flagIsMenuedGroup != 0 {
			updateMenuedGroup(view, item)
			continue
		}
		if f.Flags&flagIsButton != 0 {
			enabled, is := item.Interface.(bool)
			if is {
				view.SetUsable(enabled)
			}
			continue
		}
		if menu == nil && f.Kind == zreflect.KindSlice && item.Value.Len() > 0 {
			if updateStackFromActionFieldHandlerSlice(view, &item, f) {
				continue
			}
		}

		switch f.Kind {
		case zreflect.KindSlice:
			items, got := item.Interface.(zdict.NamedValues)
			if got {
				menu := view.(*zui.MenuView)
				menu.UpdateValues(items)
			} else if f.IsStatic() {
				mItems, _ := item.Interface.(zdict.NamedValues)
				if mItems != nil {
					menu := view.(*zui.MenuView)
					menu.UpdateValues(mItems)
				}
			}

		case zreflect.KindTime:
			str := getTimeString(item, f)
			to := view.(zui.TextLayoutOwner)
			to.SetText(str)

		case zreflect.KindStruct:
			_, got := item.Interface.(zui.UIStringer)
			if got {
				break
			}
			fv, _ := view.(*FieldView)
			if fv == nil {
				break
			}
			fv.fieldOwner.updateStack(&fv.StackView, "")
			break

		case zreflect.KindBool:
			b := zbool.ToBoolInd(item.Value.Interface().(bool))
			cv := view.(*zui.CheckBox)
			v := cv.Value()
			if v != b {
				cv.SetValue(b)
			}

		case zreflect.KindInt, zreflect.KindFloat:
			tv, _ := view.(*zui.TextView)
			if tv != nil {
				str := getTextFromNumberishItem(item, f)
				tv.SetText(str)
			}

		case zreflect.KindString, zreflect.KindFunc:
			str := item.Value.String()
			// zlog.Info("Update stack:", i, f.Name, f.Kind, str)
			if f.Flags&flagIsImage != 0 {
				path := ""
				if f.Kind == zreflect.KindString {
					path = str
				}
				if path != "" && strings.Contains(f.FixedPath, "*") {
					path = strings.Replace(f.FixedPath, "*", path, 1)
				} else if path == "" || f.Flags&flagImageIsFixed != 0 {
					path = f.FixedPath
				}
				iv := view.(*zui.ImageView)
				iv.SetImage(nil, path, nil)
			} else {
				if f.IsStatic() {
					label, _ := view.(*zui.Label)
					zlog.Assert(label != nil)
					label.SetText(str)
				} else {
					tv, _ := view.(*zui.TextView)
					// zlog.Info("fields set text:", f.Name, str)
					if tv != nil {
						tv.SetText(str)
					}
				}
			}
		}
	}
	// call general one with no id. Needs to be after above loop, so values set
	fh, _ := fo.structure.(ActionHandler)
	if fh != nil {
		sview := stack.View
		fh.HandleAction("", structID, nil, DataChangedAction, &sview)
	}
}

func findLocalField(children *[]zreflect.Item, name string) *zreflect.Item {
	name = zstr.HeadUntil(name, ".")
	for i, c := range *children {
		if c.FieldName == name {
			return &(*children)[i]
		}
	}
	return nil
}

func updateMenuedGroup(view zui.View, item zreflect.Item) {
	id := zstr.FirstToLowerWithAcronyms(item.FieldName)

	fv, _ := zui.ViewChild(view, id).(*FieldView)

	key := makeMenuedGroupKey(id, item)

	zlog.Info("updateMenuedGroup:", key, "\n", zlog.GetCallingStackString())
	fv.fieldOwner.structure = getSliceElementOfMenuedGroup(item, key)
	fv.Update()
}

func refreshMenuedGroup(key, name string, menu *zui.MenuView, fieldView *FieldView, item interface{}) {
	menu.Empty()

	mItems, _ := item.(zdict.NamedValues)

	// zlog.Info("refreshMenuedGroup:", mItems)

	menu.AddAction("$title", name+":")
	menu.AddAction("$add", "Add")

	c := mItems.Count()
	if c > 0 {
		menu.AddAction("$remove", "Remove Current")
		menu.AddSeparator()
	}
	fieldView.Show(c != 0)
	menu.SetValues(mItems, nil)

	//!!! menu.updateVals(mItems, nil)
	currentID, _ := zui.DefaultLocalKeyValueStore.StringForKey(key)
	// zlog.Info("Set current:", currentID)
	if currentID != "" {
		menu.SetWithID(currentID)
	} else {
		menu.SetWithID("$title")
	}
}

func makeMenuedGroupKey(id string, item zreflect.Item) string {
	zlog.Info("makeMenuedGroupKey:", id, item.FieldName)
	return id + "." + item.FieldName + ".MenuedGroupIndex"
}

func getSliceIndexOfMenuedGroup(item zreflect.Item, idKey string) int {
	mItems, _ := item.Interface.(zdict.NamedValues)
	zlog.Info("getSliceIndexOfMenuedGroup:", item.Interface, mItems)
	currentID, _ := zui.DefaultLocalKeyValueStore.StringForKey(idKey)
	return zdict.NamedValuesIndexOfID(mItems, currentID)
}

func getSliceElementOfMenuedGroup(item zreflect.Item, idKey string) interface{} {
	iv := item.Value
	if iv.Len() == 0 {
		return reflect.New(iv.Type().Elem()).Interface()
	}
	index := getSliceIndexOfMenuedGroup(item, idKey)
	zlog.Info("getSliceElementOfMenuedGroup", index)
	if index == -1 {
		index = 0
	}
	return iv.Index(index).Addr().Interface()
}

func (fo *fieldOwner) makeMenuedGroup(stack *zui.StackView, item zreflect.Item, f *Field, structID string, defaultAlign zgeo.Alignment, cellMargin zgeo.Size) zui.View {
	// zlog.Info("makeMenuedGroup", f.Name, f.LabelizeWidth)
	var fv *FieldView
	vert := zui.StackViewVert("mgv")

	vert.SetCorner(5)
	vert.SetStroke(4, zgeo.ColorNewGray(0, 0.2))

	menu := zui.MenuViewNew("menu", nil, nil, false)

	vert.Add(zgeo.Left|zgeo.Top, menu, zgeo.Size{6, -6})

	key := makeMenuedGroupKey(fo.id, item)

	s := getSliceElementOfMenuedGroup(item, key)

	id := zstr.FirstToLowerWithAcronyms(item.FieldName)
	fv = FieldViewNew(id, s, f.LabelizeWidth)
	fv.fieldOwner.handleUpdate = func(edited bool, structID string) {
		refreshMenuedGroup(key, item.FieldName, menu, fv, item.Interface)
		// zlog.Info("updated!", fv.structure, item.Interface)
	}
	fv.Build(false) // don't update here, we do below?
	vert.Add(zgeo.Left|zgeo.Top|zgeo.Expand, fv)

	refreshMenuedGroup(key, item.FieldName, menu, fv, item.Interface)

	menu.ChangedHandler(func(id string, name string, value interface{}) {
		// zlog.Info("makeMenuedGroup changed:", id)
		switch id {
		case "$add":
			iv := item.Value
			a := reflect.New(iv.Type().Elem()).Elem()
			nv := reflect.Append(iv, a)
			iv.Addr().Elem().Set(nv)
			item.Value = iv
			item.Address = item.Value.Addr()
			item.Interface = item.Value.Interface()
			added := iv.Index(iv.Len() - 1)
			fv.structure = added.Addr().Interface()
			fo.callActionHandlerFunc(structID, f, CreateAction, item.Interface, nil)
			zui.DefaultLocalKeyValueStore.SetInt(iv.Len()-1, key, true)
			//			fv.structure = getSliceElementOfMenuedGroup(item, key)
			fv.updateStack(&fv.StackView, "")
			refreshMenuedGroup(key, item.FieldName, menu, fv, iv.Interface())

		case "$remove":
			index := getSliceIndexOfMenuedGroup(item, key)
			if index != -1 {
				zslice.RemoveAt(item.Address, index)
				item.Interface = item.Value.Interface()
			}
			currentID := "$title"
			zint.Minimize(&index, item.Value.Len()-1)
			zlog.Info("REMOVE1:", index, item.Interface)
			zlog.Info("REMOVE:", index, item.Value.Len(), reflect.ValueOf(item.Address).Elem().Len())
			if item.Value.Len() > 0 {
				currentID = strconv.Itoa(index)
				// s := item.Value.Index(0).Addr().Interface()
			}
			zui.DefaultLocalKeyValueStore.SetString(currentID, key, true)
			fv.structure = getSliceElementOfMenuedGroup(item, key)
			fv.updateStack(&fv.StackView, "")
			refreshMenuedGroup(key, item.FieldName, menu, fv, item.Interface)
			fo.callActionHandlerFunc(structID, f, EditedAction, item.Interface, nil)
			break

		case "$title":
			break

		default:
			zui.DefaultLocalKeyValueStore.SetString(id, key, true)
			fv.structure = getSliceElementOfMenuedGroup(item, key)
			zlog.Info("Changed menued group to id:", id, fv.structure, "to:", fo.structure)
			fv.updateStack(&fv.StackView, "")
			fo.callActionHandlerFunc(structID, f, EditedAction, item.Interface, nil)
			break
		}
	})

	return vert
}

func createStackFromActionFieldHandlerSlice(item *zreflect.Item, f *Field) zui.View {
	var views []zui.View
	for n := 0; n < item.Value.Len(); n++ {
		h, _ := item.Value.Index(0).Interface().(ActionFieldHandler)
		if h != nil {
			// zlog.Info("SLICE:", f.Name, f.Kind, len(item.Children))
			var sview zui.View
			ah := item.Value.Index(n).Interface().(ActionFieldHandler)
			if ah.HandleFieldAction(f, CreateAction, &sview) && sview != nil {
				views = append(views, sview)
			}
		}
	}
	if len(views) == 0 {
		return nil
	}
	stack := zui.StackViewHor(f.Name + ".stack")
	if f.Spacing != 0 {
		stack.SetSpacing(f.Spacing)
	}
	for _, v := range views {
		stack.Add(zgeo.Left|zgeo.VertCenter, v)
	}
	// updateStackFromActionFieldHandlerSlice(stack, item, f)
	return stack
}

func updateStackFromActionFieldHandlerSlice(view zui.View, item *zreflect.Item, f *Field) bool {
	var updated bool
	ct, _ := view.(zui.ContainerType)
	if ct == nil {
		return false
	}
	views := ct.GetChildren()
	for n := 0; n < item.Value.Len(); n++ {
		if len(views) <= n {
			break
		}
		h, _ := item.Value.Index(0).Interface().(ActionFieldHandler)
		if h != nil {
			updated = true
			sview := views[n]
			ah := item.Value.Index(n).Interface().(ActionFieldHandler)
			ah.HandleFieldAction(f, DataChangedAction, &sview)
		}
	}
	return updated
}

func (fo *fieldOwner) buildStack(name string, stack *zui.StackView, parentField *Field, fields *[]Field, defaultAlign zgeo.Alignment, cellMargin zgeo.Size, useMinWidth bool, inset float64, structID string) {
	zlog.Assert(reflect.ValueOf(fo.structure).Kind() == reflect.Ptr, name, fo.structure)
	// fmt.Printf("buildStack1 %s %s %+v\n", name, structID, fo.structure)
	children := fo.getStructItems(structID, true)
	labelizeWidth := fo.labelizeWidth
	if parentField != nil && fo.labelizeWidth == 0 {
		labelizeWidth = parentField.LabelizeWidth
	}
	// zlog.Info("buildStack", name, structID, len(children))
	for j, item := range children {
		exp := zgeo.AlignmentNone
		// zlog.Info("   buildStack1.2", j, item)
		f := findFieldWithIndex(fields, j)
		if f == nil {
			//			zlog.Error(nil, "no field for index", j)
			continue
		}
		// zlog.Info("   buildStack2", j, f.Name, item)

		var view zui.View
		if f.Flags&flagIsButton != 0 {
			view = fo.makeButton(item, f, structID)
		} else if f.Flags&flagIsMenuedGroup != 0 {
			view = fo.makeMenuedGroup(stack, item, f, structID, defaultAlign, cellMargin)
		} else {
			fo.callActionHandlerFunc(structID, f, CreateAction, item.Interface, &view) // this sees if actual ITEM is a field handler
			// if called {
			// 	zlog.Info("CALLED:", f.Name, view)
			// }
		}
		if view == nil && f.Kind == zreflect.KindSlice && item.Value.Len() > 0 {
			view = createStackFromActionFieldHandlerSlice(&item, f)
		}
		if view != nil {
		} else if f.LocalEnum != "" {
			ei := findLocalField(&children, f.LocalEnum)
			if !zlog.ErrorIf(ei == nil, f.Name, f.LocalEnum) {
				enum, _ := ei.Interface.(zdict.NamedValues)
				// zlog.Info("make local enum:", f.Name, f.LocalEnum, i, enum, ei)
				if zlog.ErrorIf(enum == nil, "field isn't enum, not NamedValues type", f.Name, f.LocalEnum) {
					continue
				}
				// zlog.Info("make local enum:", f.Name, f.LocalEnum, i, MenuItemsLength(enum))
				menu := fo.makeMenu(item, f, structID, enum)
				if menu == nil {
					zlog.Error(nil, "no local enum for", f.LocalEnum)
					continue
				}
				view = menu
				menu.SetWithIdOrValue(item.Interface)
			}
		} else if f.Enum != "" {
			//			fmt.Printf("make enum: %s %v\n", f.Name, item)
			enum, _ := fieldEnums[f.Enum]
			zlog.Assert(enum != nil)
			view = fo.makeMenu(item, f, structID, enum)
			exp = zgeo.AlignmentNone
		} else {
			switch f.Kind {
			case zreflect.KindStruct:
				_, got := item.Interface.(zui.UIStringer)
				// zlog.Info("make stringer?:", f.Name, got)
				if got && f.IsStatic() {
					view = fo.makeText(item, f, structID)
				} else {
					exp = zgeo.HorExpand
					// zlog.Info("struct make field view:", f.Name, f.Kind, exp)
					childStruct := item.Address
					vertical := true
					fieldView := fieldViewNew(f.ID, vertical, childStruct, 10, zgeo.Size{}, labelizeWidth)
					fieldView.parentField = f
					view = fieldView
					fieldView.buildStack(f.ID, &fieldView.StackView, fieldView.parentField, &fieldView.fields, zgeo.Left|zgeo.Top, zgeo.Size{}, true, 5, "")
				}

			case zreflect.KindBool:
				b := zbool.ToBoolInd(item.Value.Interface().(bool))
				view = fo.makeCheckbox(item, f, structID, b)

			case zreflect.KindInt:
				if item.TypeName == "BoolInd" {
					exp = zgeo.HorShrink
					view = fo.makeCheckbox(item, f, structID, zbool.BoolInd(item.Value.Int()))
				} else {
					view = fo.makeText(item, f, structID)
				}

			case zreflect.KindFloat:
				view = fo.makeText(item, f, structID)

			case zreflect.KindString:
				if f.Flags&flagIsImage != 0 {
					// zlog.Info("Make ImageField:", f.Name, f.Size)
					view = fo.makeImage(item, f, structID)
				} else {
					exp = zgeo.HorExpand
					view = fo.makeText(item, f, structID)
				}

			case zreflect.KindSlice:
				items, got := item.Interface.(zdict.NamedValues)
				if got {
					menu := fo.makeMenu(item, f, structID, items)
					view = menu
					break
				}
				view = fo.makeText(item, f, structID)
				break

			case zreflect.KindTime:
				view = fo.makeText(item, f, structID)

			default:
				panic(fmt.Sprintln("buildStack bad type:", f.Name, f.Kind))
			}
		}
		pt, _ := view.(zui.Pressable)
		if pt != nil {
			ph := pt.PressedHandler()
			nowItem := item // store item in nowItem so closures below uses right item
			pt.SetPressedHandler(func() {
				if !fo.callActionHandlerFunc(structID, f, PressedAction, nowItem.Interface, &view) && ph != nil {
					ph()
				}
			})
			lph := pt.LongPressedHandler()
			pt.SetLongPressedHandler(func() {
				if !fo.callActionHandlerFunc(structID, f, LongPressedAction, nowItem.Interface, &view) && lph != nil {
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
			stack.AddView(lstack, zgeo.HorExpand|zgeo.Left|zgeo.Top)
		}
		cell.Margin = cellMargin
		def := defaultAlign
		all := zgeo.Left | zgeo.HorCenter | zgeo.Right
		if f.Alignment&all != 0 {
			def &= ^all
		}
		cell.Alignment = def | exp | f.Alignment
		//  zlog.Info("field align:", f.Alignment, f.Name, def|exp, cell.Alignment, int(cell.Alignment))
		cell.Weight = f.Weight
		if parentField != nil && cell.Weight == 0 {
			cell.Weight = parentField.DefaultWeight
		}
		if useMinWidth {
			cell.MinSize.W = f.MinWidth
			// zlog.Info("Cell Width:", f.Name, cell.MinSize.W)
		}
		cell.MaxSize.W = f.MaxWidth
		if cell.MinSize.W != 0 && (j == 0 || j == len(children)-1) {
			cell.MinSize.W += inset
		}
		// zlog.Info("Add Field Item:", cell.View.ObjectName(), cell.Alignment, f.MinWidth, cell.MinSize.W, cell.MaxSize)
		if labelizeWidth == 0 {
			cell.View = view
			stack.AddCell(*cell, -1)
		}
	}
	// zlog.Info("buildStack END", name, structID)

}

func (f *Field) makeFromReflectItem(fo *fieldOwner, structure interface{}, item zreflect.Item, index int) bool {
	f.Index = index
	f.ID = zstr.FirstToLowerWithAcronyms(item.FieldName)
	f.Kind = item.Kind
	f.FieldName = item.FieldName
	f.Alignment = zgeo.AlignmentNone
	f.UpdateSecs = 4

	// zlog.Info("Field:", f.ID)
	for _, part := range zreflect.GetTagAsMap(item.Tag)["zui"] {
		if part == "-" {
			return false
		}
		var key, val string
		if !zstr.SplitN(part, ":", &key, &val) {
			key = part
		}
		key = strings.TrimSpace(key)
		origVal := val
		val = strings.TrimSpace(val)
		align := zgeo.AlignmentFromString(val)
		n, floatErr := strconv.ParseFloat(val, 32)
		flag := zbool.FromString(val, false)
		switch key {
		case "align":
			if align != zgeo.AlignmentNone {
				f.Alignment = align
			}
			// zlog.Info("ALIGN:", f.Name, val, a)
		case "justify":
			if align != zgeo.AlignmentNone {
				f.Justify = align
			}
		case "name":
			f.Name = origVal
		case "title":
			f.Title = origVal
		case "color":
			f.Colors = strings.Split(val, "|")
		case "height":
			if floatErr == nil {
				f.Height = n
			}
		case "weight":
			if floatErr == nil {
				f.Weight = n
			}
		case "weights":
			if floatErr == nil {
				f.DefaultWeight = n
			}
		case "width":
			if floatErr == nil {
				f.MinWidth = n
				f.MaxWidth = n
			}
		case "minwidth":
			if floatErr == nil {
				f.MinWidth = n
			}
		case "spacing":
			if floatErr == nil {
				f.Spacing = n
			}
		case "static":
			if flag || val == "" {
				f.Flags |= flagIsStatic
			}
		case "secs":
			f.Flags |= flagHasSeconds
		case "mins":
			f.Flags |= flagHasMinutes
		case "hours":
			f.Flags |= flagHasHours
		case "maxwidth":
			if floatErr == nil {
				f.MaxWidth = n
			}
		case "fixed":
			f.Flags |= flagImageIsFixed
		case "font":
			var sign int
			for _, part := range strings.Split(val, "|") {
				if zstr.HasPrefix(part, "+", &part) {
					sign = 1
				}
				if zstr.HasPrefix(part, "-", &part) {
					sign = -1
				}
				n, _ := strconv.Atoi(part)
				if n != 0 {
					if sign != 0 {
						f.FontSize = float64(n*sign) + zui.FontDefaultSize
					} else {
						f.FontSize = float64(n)
					}
				} else {
					if f.FontName == "" && f.FontSize == 0 {
						f.FontName = part
					} else {
						f.FontStyle = zui.FontStyleFromStr(part)
					}
				}
			}

		case "image", "himage":
			var ssize, path string
			if !zstr.SplitN(val, "|", &ssize, &path) {
				ssize = val
			} else {
				f.FixedPath = "images/" + path
			}
			if key == "image" {
				f.Flags |= flagIsImage
			} else {
				f.Flags |= flagHasHeaderImage
			}
			f.Size.FromString(ssize)
		case "enum":
			if zstr.HasPrefix(val, ".", &f.LocalEnum) {
			} else {
				if fieldEnums[val] == nil {
					zlog.Error(nil, "no such enum:", val)
				}
				f.Enum = val
			}
		case "notitle":
			f.Flags |= flagNoTitle
		case "tip":
			f.Tooltip = val
		case "immediate":
			f.UpdateSecs = 0
		case "upsecs":
			if floatErr == nil && n > 0 {
				f.UpdateSecs = n
			}
		case "2clip":
			f.Flags |= flagToClipboard
		case "menued-group":
			mi, _ := item.Interface.(zdict.NamedValues)
			zlog.Assert(mi != nil)
			f.Flags |= flagIsMenuedGroup
		case "labelize":
			f.LabelizeWidth = n
			if n == 0 {
				f.LabelizeWidth = 200
			}
		case "button":
			f.Flags |= flagIsButton
		case "enable":
			if !zstr.HasPrefix(val, ".", &f.LocalEnable) {
				zlog.Error(nil, "fields enable: only dot prefix local fields allowed")
			}
		}
	}
	if f.Flags&flagToClipboard != 0 && f.Tooltip == "" {
		f.Tooltip = "press to copy to Clipboard"
	}
	zfloat.Maximize(&f.MaxWidth, f.MinWidth)
	zfloat.Minimize(&f.MinWidth, f.MaxWidth)
	if f.Name == "" {
		str := zstr.PadCamelCase(item.FieldName, " ")
		str = zstr.FirstToTitleCase(str)
		f.Name = str
	}

	switch item.Kind {
	case zreflect.KindFloat:
		if f.MinWidth == 0 {
			f.MinWidth = 64
		}
		if f.MaxWidth == 0 {
			f.MaxWidth = 64
		}
	case zreflect.KindInt:
		if item.TypeName != "BoolInd" {
			if item.Package == "time" && item.TypeName == "Duration" {
				if f.Flags&flagTimeFlags == 0 {
					f.Flags |= flagTimeFlags
				}
			}
			if f.MinWidth == 0 {
				f.MinWidth = 60
			}
			if f.MaxWidth == 0 {
				f.MaxWidth = 80
			}
			break
		}
		fallthrough

	case zreflect.KindBool:
		if f.MinWidth == 0 {
			f.MinWidth = 20
		}
	case zreflect.KindString:
		if f.Flags&(flagHasHeaderImage|flagIsImage) != 0 {
			f.MinWidth = f.Size.W
			f.MaxWidth = f.Size.W
		}
		if f.MinWidth == 0 && f.Flags&flagIsButton == 0 {
			f.MinWidth = 100
		}
	case zreflect.KindTime:
		if f.MinWidth == 0 {
			f.MinWidth = 80
		}
		if f.Flags&(flagTimeFlags|flagDateFlags) == 0 {
			f.Flags |= flagTimeFlags | flagDateFlags
		}
		dig2 := 20.0
		if f.MinWidth == 0 {
			if f.Flags&flagHasSeconds != 0 {
				f.MaxWidth += dig2

			}
			if f.Flags&flagHasMinutes != 0 {
				f.MaxWidth += dig2

			}
			if f.Flags&flagHasHours != 0 {
				f.MaxWidth += dig2

			}
			if f.Flags&flagHasDays != 0 {
				f.MaxWidth += dig2

			}
			if f.Flags&flagHasMonths != 0 {
				f.MaxWidth += dig2

			}
			if f.Flags&flagHasYears != 0 {
				f.MaxWidth += dig2 * 2
			}
		}
		//		zlog.Info("Time max:", f.MaxWidth)

	case zreflect.KindFunc:
		if f.MinWidth == 0 {
			if f.Flags&flagIsImage != 0 {
				min := f.Size.W // * ScreenMain().Scale
				//				min += ImageViewDefaultMargin.W * 2
				zfloat.Maximize(&f.MinWidth, min)
			}
		}
	}
	fo.callActionHandlerFunc("", f, SetupAction, item.Interface, nil) // need to use fo.structure here, since i == -1
	return true
}

func (fo *fieldOwner) fieldViewToDataItem(structID string, f *Field, view zui.View, showError bool) error {
	var err error

	if f.Flags&flagIsStatic != 0 {
		return nil
	}
	children := fo.getStructItems(structID, true)
	// zlog.Info("fieldViewToDataItem before:", f.Name, f.Index, len(children), "s:", structure)
	item := children[f.Index]
	if (f.Enum != "" || f.LocalEnum != "") && !f.IsStatic() {
		mv, _ := view.(*zui.MenuView)
		if mv != nil {
			iface := mv.GetCurrentIdOrValue()
			// zlog.Debug(iface, f.Name)
			item.Value.Set(reflect.ValueOf(iface))
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
			*b = bv.Value().Value()
		}
		bi, _ := item.Address.(*zbool.BoolInd)
		if bi != nil {
			*bi = bv.Value()
		}

	case zreflect.KindInt:
		if !f.IsStatic() {
			if item.TypeName == "BoolInd" {
				bv, _ := view.(*zui.CheckBox)
				*item.Address.(*bool) = bv.Value().Value()
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

var fieldEnums = map[string]zdict.NamedValues{}

func SetEnum(name string, enum zdict.NamedValues) {
	fieldEnums[name] = enum
}

func SetEnumItems(name string, nameValPairs ...interface{}) {
	var dis zdict.Items

	for i := 0; i < len(nameValPairs); i += 2 {
		var di zdict.Item
		di.Name = nameValPairs[i].(string)
		di.Value = nameValPairs[i+1]
		dis = append(dis, di)
	}
	fieldEnums[name] = dis
}

func AddStringBasedEnum(name string, vals ...interface{}) {
	var items zdict.Items
	for _, v := range vals {
		n := fmt.Sprintf("%v", v)
		i := zdict.Item{n, v}
		items = append(items, i)
	}
	fieldEnums[name] = items
}
