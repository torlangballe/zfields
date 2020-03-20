package zfields

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/torlangballe/zui"
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
	Title         string // name of item in row, and header if no title
	Width         float64
	MaxWidth      float64
	MinWidth      float64
	Kind          zreflect.TypeKind
	Alignment     zgeo.Alignment
	Justify       zgeo.Alignment
	Format        string
	Color         string
	FixedPath     string
	Password      bool
	Height        float64
	Weight        float64
	Enum          zdict.NamedValues
	LocalEnum     string
	Size          zgeo.Size
	Flags         int
	DefaultWeight float64
	Tooltip       string
	UpdateSecs    float64
	LabelizeWidth float64
	LocalEnable   string
	// Type          fieldType
}

// FieldStringer is an interface that allows simple structs to be displayed like labels in fields and menus.
// We don't want to use String() as complex structs can have these for debugging
type ActionHandler interface {
	ZHandleAction(id string, i int, f *Field, action ActionType, view *zui.View) bool
}

type ActionFieldHandler interface {
	ZHandleAction(f *Field, action ActionType, view *zui.View) bool
}

type fieldOwner struct {
	fields        []Field
	structure     interface{} // structure of ALL, not just a row
	id            string
	handleUpdate  func(edited bool, i int)
	labelizeWidth float64
}

func callActionHandlerFunc(structure interface{}, i int, f *Field, action ActionType, item zreflect.Item, view *zui.View) bool {
	fh, _ := structure.(ActionHandler)
	// fmt.Println("callFieldHandler1", fh)
	var result bool
	if fh != nil {
		result = fh.ZHandleAction(f.ID, i, f, action, view)
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
							fh2.ZHandleAction(id2, i, f2, action, &parent.View)
						}
					}
					first = false
				}
			}
			n = parent
		}
	}
	if !result {
		aih, _ := item.Interface.(ActionFieldHandler)
		// fmt.Println("callActionHandlerFunc:", f.Name, action, aih, item)
		if aih != nil {
			result = aih.ZHandleAction(f, action, view)
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

type FieldView struct {
	zui.StackView
	fieldOwner
	parentField *Field
}

func (f Field) IsStatic() bool {
	return f.Flags&flagIsStatic != 0
}

func makeButton(structure interface{}, item zreflect.Item, f *Field, i int) *zui.Button {
	// fmt.Println("makeButton:", f.Name, f.Height)
	format := f.Format
	if format == "" {
		format = "%v"
	}
	color := f.Color
	if color == "" {
		color = "gray"
	}
	name := f.Name
	if f.Title != "" {
		name = f.Title
	}
	button := zui.ButtonNew(name, color, zgeo.Size{40, f.Height}, zgeo.Size{}) //ShapeViewNew(ShapeViewTypeRoundRect, s)
	button.TextInfo.Color = zgeo.ColorRed
	button.TextXMargin = 0
	button.SetPressedHandler(func() {
		fmt.Println("Button pressed", button.ObjectName(), button.Font())
		view := zui.View(button)
		callActionHandlerFunc(structure, i, f, PressedAction, item, &view)
	})
	return button
}

func makeMenu(structure interface{}, item zreflect.Item, f *Field, i int, items zdict.NamedValues) *zui.MenuView {
	menu := zui.MenuViewNew(f.Name+"Menu", items, item.Interface, f.IsStatic())
	menu.SetMaxWidth(f.MaxWidth)

	// fmt.Println("makeMenu2:", f.Name, items.Count(), item.Interface, item.TypeName, item.Kind)

	menu.ChangedHandler(func(id, name string, value interface{}) {
		//		zlog.Debug(iface, f.Name)
		fieldViewToDataItem(structure, f, menu, false)
		//		item.Value.Set(reflect.ValueOf(iface))
		callActionHandlerFunc(structure, i, f, EditedAction, item, nil)
	})
	return menu
}

func makeTimeView(structure interface{}, item zreflect.Item, f *Field, i int) zui.View {
	var str string
	t := item.Interface.(time.Time)
	format := f.Format
	if format == "" {
		format = "15:04 01-Jan-06"
	}
	// fmt.Println("makeTimeView:", format)
	if format == "nice" {
		str = ztime.GetNice(t, f.Flags&flagHasSeconds != 0)
	} else {
		str = t.Format(format)
	}
	if f.IsStatic() {
		label := zui.LabelNew(str)
		return label
	}
	var style zui.TextViewStyle
	tv := zui.TextViewNew(str, style)
	return tv
}

func makeText(fo *fieldOwner, structure interface{}, item zreflect.Item, f *Field, i int) zui.View {
	// fmt.Println("make Text:", item.FieldName, f.Name, fo.structure)
	str := ""
	if item.Package == "time" && item.TypeName == "Duration" {
		t := ztime.DurSeconds(time.Duration(item.Value.Int()))
		str = ztime.GetSecsAsHMSString(t, f.Flags&flagHasHours != 0, f.Flags&flagHasMinutes != 0, f.Flags&flagHasSeconds != 0, 0)
	} else {
		format := f.Format
		if format == "" {
			format = "%v"
		}
		str = fmt.Sprintf(format, item.Value.Interface())
	}
	if f.IsStatic() {
		label := zui.LabelNew(str)
		j := f.Justify
		if j == zgeo.AlignmentNone {
			j = f.Alignment & (zgeo.Left | zgeo.HorCenter | zgeo.Right)
			if j == zgeo.AlignmentNone {
				j = zgeo.Left
			}
		}
		label.SetTextAlignment(j)
		label.SetPressedHandler(func() {
			view := zui.View(label)
			callActionHandlerFunc(structure, i, f, PressedAction, item, &view)
		})
		if f.Flags&flagToClipboard != 0 {
			label.SetPressedHandler(func() {
				text := label.Text()
				zui.PasteboardSetString(text)
				label.SetText("📋 " + text)
				ztimer.StartIn(0.6, true, func() {
					label.SetText(text)
				})
			})
		}
		return label
	}
	var style zui.TextViewStyle
	tv := zui.TextViewNew(str, style)
	tv.UpdateSecs = f.UpdateSecs
	tv.ChangedHandler(func(view zui.View) {
		fieldViewToDataItem(structure, f, tv, true)
		// fmt.Println("Changed text1:", structure)
		if fo.handleUpdate != nil {
			edited := true
			fo.handleUpdate(edited, i)
		}
		// fmt.Println("Changed text:", structure)
		view = zui.View(tv)
		callActionHandlerFunc(structure, i, f, EditedAction, item, &view)
	})
	tv.KeyHandler(func(view zui.View, key zui.KeyboardKey, mods zui.KeyboardModifier) {
		fmt.Println("keyup!")
	})
	return tv
}

func makeCheckbox(structure interface{}, item zreflect.Item, f *Field, i int, b zui.BoolInd) zui.View {
	cv := zui.CheckBoxNew(b)
	cv.SetObjectName(f.ID)
	cv.ValueHandler(func(v zui.View) {
		fieldViewToDataItem(structure, f, cv, true)
		view := zui.View(cv)
		callActionHandlerFunc(structure, i, f, EditedAction, item, &view)
	})
	return cv
}

func fieldsMakeImage(structure interface{}, item zreflect.Item, f *Field, i int) zui.View {
	iv := zui.ImageViewNew("", f.Size)
	iv.SetMaxSize(f.Size)
	iv.SetObjectName(f.ID)
	iv.SetPressedHandler(func() {
		view := zui.View(iv)
		callActionHandlerFunc(structure, i, f, PressedAction, item, &view)
	})
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

func getStructItems(structData interface{}) []zreflect.Item {
	k := reflect.ValueOf(structData).Kind()
	zlog.Assert(k == reflect.Ptr, "not pointer", k, structData)
	unnestAnon := true
	recursive := false
	rootItems, err := zreflect.ItterateStruct(structData, unnestAnon, recursive)
	if err != nil {
		panic(err)
	}
	return rootItems.Children
}

func updateStack(fo *fieldOwner, stack *zui.StackView, structData interface{}) {
	children := getStructItems(structData)

	// fmt.Println("updateStack1:", len(rootItems.Children), len(fo.fields), len(stack.GetChildren()))
	// fmt.Println("updateStack:", stack.ObjectName(), structData)
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
		called := callActionHandlerFunc(structData, 0, f, DataChangedAction, item, &view)
		// fmt.Println("updateStack:", f.Name, f.Kind, called)
		if called {
			continue
		}
		if f.Enum != nil && f.Kind != zreflect.KindSlice || f.LocalEnum != "" {
			menu := view.(*zui.MenuView)
			// fmt.Println("FUS:", f.Name, item.Interface, item.Kind, reflect.ValueOf(item.Interface).Type())
			menu.SetWithIdOrValue(item.Interface)
			continue
		}
		if f.LocalEnable != "" {
			eItem := findLocalField(&children, f.LocalEnable)
			e, got := eItem.Interface.(bool)
			fmt.Println("updateStack localEnable:", f.Name, f.LocalEnable, e, got)
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
		switch f.Kind {
		case zreflect.KindSlice:
			items, got := item.Interface.(zdict.NamedValues)
			if got {
				menu := view.(*zui.MenuView)
				menu.UpdateValues(items)
			} else if f.IsStatic() {
				mItems, _ := item.Interface.(zdict.NamedValues)
				if !zlog.ErrorIf(mItems == nil, f.Name) {
					menu := view.(*zui.MenuView)
					menu.UpdateValues(mItems)
				}
			}

		case zreflect.KindStruct:
			_, got := item.Interface.(zui.UIStringer)
			if got {
				break
			}
			fv, _ := view.(*FieldView)
			if fv == nil {
				break
			}
			updateStack(&fv.fieldOwner, &fv.StackView, item.Address)
			break

		case zreflect.KindBool:
			b := zui.BoolIndFromBool(item.Value.Interface().(bool))
			cv := view.(*zui.CheckBox)
			v := cv.Value()
			if v != b {
				cv.SetValue(b)
			}

		case zreflect.KindString, zreflect.KindFunc:
			str := item.Value.String()
			if f.Flags&flagIsImage != 0 {
				path := ""
				if f.Kind == zreflect.KindString {
					path = str
				}
				if path == "" {
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
					// fmt.Println("fields set text:", f.Name, str)
					if tv != nil {
						tv.SetText(str)
					}
				}
			}
		}
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

	fmt.Println("updateMenuedGroup:", key, "\n", zlog.GetCallingStackString())
	fv.fieldOwner.structure = getSliceElementOfMenuedGroup(item, key)
	fv.Update()
}

func refreshMenuedGroup(key, name string, menu *zui.MenuView, fieldView *FieldView, item interface{}) {
	menu.Empty()

	mItems, _ := item.(zdict.NamedValues)

	// fmt.Println("refreshMenuedGroup:", mItems)

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
	// fmt.Println("Set current:", currentID)
	if currentID != "" {
		menu.SetWithID(currentID)
	} else {
		menu.SetWithID("$title")
	}
}

func makeMenuedGroupKey(id string, item zreflect.Item) string {
	fmt.Println("makeMenuedGroupKey:", id, item.FieldName)
	return id + "." + item.FieldName + ".MenuedGroupIndex"
}

func getSliceIndexOfMenuedGroup(item zreflect.Item, idKey string) int {
	mItems, _ := item.Interface.(zdict.NamedValues)
	fmt.Println("getSliceIndexOfMenuedGroup:", item.Interface, mItems)
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

func makeMenuedGroup(fo *fieldOwner, stack *zui.StackView, item zreflect.Item, f *Field, i int, defaultAlign zgeo.Alignment, cellMargin zgeo.Size) zui.View {
	// fmt.Println("makeMenuedGroup", f.Name, f.LabelizeWidth)
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
	fv.fieldOwner.handleUpdate = func(edited bool, i int) {
		refreshMenuedGroup(key, item.FieldName, menu, fv, item.Interface)
		// fmt.Println("updated!", fv.structure, item.Interface)
	}
	fv.Build(false) // don't update here, we do below?
	vert.Add(zgeo.Left|zgeo.Top|zgeo.Expand, fv)

	refreshMenuedGroup(key, item.FieldName, menu, fv, item.Interface)

	menu.ChangedHandler(func(id string, name string, value interface{}) {
		// fmt.Println("makeMenuedGroup changed:", id)
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
			callActionHandlerFunc(fv.structure, i, f, CreateAction, item, nil)
			zui.DefaultLocalKeyValueStore.SetInt(iv.Len()-1, key, true)
			//			fv.structure = getSliceElementOfMenuedGroup(item, key)
			updateStack(&fv.fieldOwner, &fv.StackView, fv.structure)
			refreshMenuedGroup(key, item.FieldName, menu, fv, iv.Interface())

		case "$remove":
			index := getSliceIndexOfMenuedGroup(item, key)
			if index != -1 {
				zslice.RemoveAt(item.Address, index)
				item.Interface = item.Value.Interface()
			}
			currentID := "$title"
			zint.Minimize(&index, item.Value.Len()-1)
			fmt.Println("REMOVE1:", index, item.Interface)
			fmt.Println("REMOVE:", index, item.Value.Len(), reflect.ValueOf(item.Address).Elem().Len())
			if item.Value.Len() > 0 {
				currentID = strconv.Itoa(index)
				// s := item.Value.Index(0).Addr().Interface()
			}
			zui.DefaultLocalKeyValueStore.SetString(currentID, key, true)
			fv.structure = getSliceElementOfMenuedGroup(item, key)
			updateStack(&fv.fieldOwner, &fv.StackView, fv.structure)
			refreshMenuedGroup(key, item.FieldName, menu, fv, item.Interface)
			callActionHandlerFunc(fo.structure, i, f, EditedAction, item, nil)
			break

		case "$title":
			break

		default:
			zui.DefaultLocalKeyValueStore.SetString(id, key, true)
			fv.structure = getSliceElementOfMenuedGroup(item, key)
			fmt.Println("Changed menued group to id:", id, fv.structure, "toxxxxx:", fo.structure)
			updateStack(&fv.fieldOwner, &fv.StackView, fv.structure)
			callActionHandlerFunc(fv.structure, i, f, EditedAction, item, nil)
			break
		}
	})

	return vert
}

func buildStack(name string, fo *fieldOwner, stack *zui.StackView, structData interface{}, parentField *Field, fields *[]Field, defaultAlign zgeo.Alignment, cellMargin zgeo.Size, useMinWidth bool, inset float64, i int) {
	zlog.Assert(reflect.ValueOf(structData).Kind() == reflect.Ptr, name, structData)
	children := getStructItems(structData)
	labelizeWidth := fo.labelizeWidth
	if parentField != nil && fo.labelizeWidth == 0 {
		labelizeWidth = parentField.LabelizeWidth
	}
	// fmt.Println("buildStack", name, structData, fo.structure)
	for j, item := range children {
		exp := zgeo.AlignmentNone
		f := findFieldWithIndex(fields, j)
		if f == nil {
			//			zlog.Error(nil, "no field for index", j)
			continue
		}
		// fmt.Println("   buildStack2", j, f.Name, f.MinWidth)

		var view zui.View
		if f.Flags&flagIsButton != 0 {
			view = makeButton(structData, item, f, i)
		} else if f.Flags&flagIsMenuedGroup != 0 {
			view = makeMenuedGroup(fo, stack, item, f, i, defaultAlign, cellMargin)
		} else {
			callActionHandlerFunc(item, i, f, CreateAction, item, &view) // this sees if actual ITEM is a field handler
		}
		if view == nil {
			callActionHandlerFunc(structData, i, f, CreateAction, item, &view)
		}
		if view != nil {
		} else if f.LocalEnum != "" {
			ei := findLocalField(&children, f.LocalEnum)
			if !zlog.ErrorIf(ei == nil, f.Name, f.LocalEnum) {
				enum, _ := ei.Interface.(zdict.NamedValues)
				// fmt.Println("make local enum:", f.Name, f.LocalEnum, i, enum, ei)
				if zlog.ErrorIf(enum == nil, "field isn't enum, not NamedValues type", f.Name, f.LocalEnum) {
					continue
				}
				// fmt.Println("make local enum:", f.Name, f.LocalEnum, i, MenuItemsLength(enum))
				menu := makeMenu(structData, item, f, i, enum)
				if menu == nil {
					zlog.Error(nil, "no local enum for", f.LocalEnum)
					continue
				}
				view = menu
				menu.SetWithIdOrValue(item.Interface)
			}
		} else if f.Enum != nil {
			//			fmt.Printf("make enum: %s %v\n", f.Name, item)
			view = makeMenu(structData, item, f, i, f.Enum)
			exp = zgeo.AlignmentNone
		} else {
			switch f.Kind {
			case zreflect.KindStruct:
				_, got := item.Interface.(zui.UIStringer)
				// fmt.Println("make stringer?:", f.Name, got)
				if got && f.IsStatic() {
					view = makeText(fo, structData, item, f, i)
				} else {
					exp = zgeo.HorExpand
					// fmt.Println("struct make field view:", f.Name, f.Kind, exp)
					childStruct := item.Address
					vertical := true
					fieldView := fieldViewNew(f.ID, vertical, childStruct, 10, zgeo.Size{}, labelizeWidth)
					fieldView.parentField = f
					view = fieldView
					buildStack(f.ID, &fieldView.fieldOwner, &fieldView.StackView, fieldView.structure, fieldView.parentField, &fieldView.fields, zgeo.Left|zgeo.Top, zgeo.Size{}, true, 5, 0)
				}

			case zreflect.KindBool:
				b := zui.BoolIndFromBool(item.Value.Interface().(bool))
				view = makeCheckbox(structData, item, f, i, b)

			case zreflect.KindInt:
				if item.TypeName == "BoolInd" {
					exp = zgeo.HorShrink
					view = makeCheckbox(structData, item, f, i, zui.BoolInd(item.Value.Int()))
				} else {
					view = makeText(fo, structData, item, f, i)
				}

			case zreflect.KindFloat:
				view = makeText(fo, structData, item, f, i)

			case zreflect.KindString:
				if f.Flags&flagIsImage != 0 {
					// fmt.Println("Make ImageField:", f.Name, f.Size)
					view = fieldsMakeImage(structData, item, f, i)
				} else {
					exp = zgeo.HorExpand
					view = makeText(fo, structData, item, f, i)
				}

			case zreflect.KindSlice:
				items, got := item.Interface.(zdict.NamedValues)
				if got {
					menu := makeMenu(structData, item, f, i, items)
					view = menu
					break
				}
				view = makeText(fo, structData, item, f, i)
				break

			case zreflect.KindTime:
				view = makeTimeView(structData, item, f, i)

			default:
				panic(fmt.Sprintln("buildStack bad type:", f.Name, f.Kind))
			}
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
		//  fmt.Println("field align:", f.Alignment, f.Name, def|exp, cell.Alignment, int(cell.Alignment))
		cell.Weight = f.Weight
		if parentField != nil && cell.Weight == 0 {
			cell.Weight = parentField.DefaultWeight
		}
		if useMinWidth {
			cell.MinSize.W = f.MinWidth
			// fmt.Println("Cell Width:", f.Name, cell.MinSize.W)
		}
		cell.MaxSize.W = f.MaxWidth
		if cell.MinSize.W != 0 && (j == 0 || j == len(children)-1) {
			cell.MinSize.W += inset
		}
		// fmt.Println("Add Field Item:", cell.View.ObjectName(), cell.Alignment, f.MinWidth, cell.MinSize.W, cell.MaxSize)
		if labelizeWidth == 0 {
			cell.View = view
			stack.AddCell(*cell, -1)
		}
	}
}

func (f *Field) makeFromReflectItem(fo *fieldOwner, structure interface{}, item zreflect.Item, index int) bool {
	f.Index = index
	f.ID = zstr.FirstToLowerWithAcronyms(item.FieldName)
	f.Kind = item.Kind
	f.Alignment = zgeo.AlignmentNone
	f.UpdateSecs = 4

	// fmt.Println("Field:", f.ID)
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
		flag := zstr.StrToBool(val, false)
		switch key {
		case "align":
			if align != zgeo.AlignmentNone {
				f.Alignment = align
			}
			// fmt.Println("ALIGN:", f.Name, val, a)
		case "justify":
			if align != zgeo.AlignmentNone {
				f.Justify = align
			}
		case "name":
			f.Name = origVal
		case "title":
			f.Title = origVal
		case "color":
			f.Color = val
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
				f.Enum, _ = fieldEnums[val]
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
		f.Tooltip = "press to copy to pasteboard"
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
		//		fmt.Println("Time max:", f.MaxWidth)

	case zreflect.KindFunc:
		if f.MinWidth == 0 {
			if f.Flags&flagIsImage != 0 {
				min := f.Size.W // * ScreenMain().Scale
				//				min += ImageViewDefaultMargin.W * 2
				zfloat.Maximize(&f.MinWidth, min)
			}
		}
	}
	callActionHandlerFunc(structure, -1, f, SetupAction, item, nil)
	return true
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
	children := getStructItems(structure)
	// fmt.Println("FieldViewNew", id, len(froot.Children), labelizeWidth)
	for i, item := range children {
		var f Field
		if f.makeFromReflectItem(&v.fieldOwner, structure, item, i) {
			v.fields = append(v.fields, f)
		}
	}
	return v
}

func (v *FieldView) Build(update bool) {
	buildStack(v.ObjectName(), &v.fieldOwner, &v.StackView, v.structure, v.parentField, &v.fields, zgeo.Left|zgeo.Top, zgeo.Size{}, true, 5, 0) // Size{6, 4}
	if update {
		v.Update()
	}
}

func (v *FieldView) Update() {
	// fmt.Printf("FV Update: %+v\n", v.structure)
	updateStack(&v.fieldOwner, &v.StackView, v.structure)
}

func FieldViewNew(id string, structure interface{}, labelizeWidth float64) *FieldView {
	v := fieldViewNew(id, true, structure, 12, zgeo.Size{10, 10}, labelizeWidth)
	return v
}

func fieldViewToDataItem(structure interface{}, f *Field, view zui.View, showError bool) error {
	var err error

	if f.Flags&flagIsStatic != 0 {
		return nil
	}
	children := getStructItems(structure)
	// fmt.Println("fieldViewToDataItem before:", f.Name, f.Index, len(children), "s:", structure)
	item := children[f.Index]
	if (f.Enum != nil || f.LocalEnum != "") && !f.IsStatic() {
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
		bi, _ := item.Address.(*zui.BoolInd)
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
