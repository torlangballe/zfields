package zfields

import (
	"reflect"
	"strings"

	"github.com/torlangballe/zui"
	"github.com/torlangballe/zutil/zgeo"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zreflect"
)

var DefaultTableRowHoverColor = zgeo.ColorNew(0.7, 0.7, 1, 1)

type TableView struct {
	zui.StackView
	List          *zui.ListView
	Header        *zui.HeaderView
	ColumnMargin  float64
	RowInset      float64
	DefaultHeight float64
	HeaderHeight  float64

	SortedIndexes []int
	GetRowCount   func() int
	GetRowHeight  func(i int) float64
	GetRowData    func(i int) interface{}
	// RowUpdated   func(edited bool, i int, rowView *StackView) bool
	//	RowDataUpdated func(i int)
	HeaderPressed     func(id string)
	HeaderLongPressed func(id string)

	structure interface{}
	fields    []Field
}

func tableGetSliceRValFromPointer(structure interface{}) reflect.Value {
	rval := reflect.ValueOf(structure)
	// zlog.Info("tableGetSliceRValFromPointer:", structure, rval.Kind())
	if rval.Kind() == reflect.Ptr {
		rval = rval.Elem()
		if rval.Kind() == reflect.Slice {
			if rval.IsNil() {
				zlog.Info("tableGetSliceRValFromPointer: slice is nil. Might work if you set slice to empty rather than it being nil.")
			}
			return rval
		}
	}
	var n *int
	return reflect.ValueOf(n)
}

func TableViewNew(name string, header bool, structData interface{}) *TableView {
	v := &TableView{}
	v.StackView.Init(v, name)
	v.Vertical = true
	v.ColumnMargin = 3
	v.RowInset = 7
	v.HeaderHeight = 28
	v.DefaultHeight = 34
	v.structure = structData

	var structure interface{}
	rval := tableGetSliceRValFromPointer(structData)
	if !rval.IsNil() {
		v.GetRowCount = func() int {
			return tableGetSliceRValFromPointer(structData).Len()
		}
		v.GetRowData = func(i int) interface{} {
			val := tableGetSliceRValFromPointer(structData)
			if val.Len() != 0 {
				return val.Index(i).Addr().Interface()
			}
			return nil
		}
		if rval.Len() == 0 {
			structure = reflect.New(rval.Type().Elem()).Interface()
		} else {
			structure = rval.Index(0).Addr().Interface()
		}
		// v.getSubStruct = func(structID string, direct bool) interface{} {
		// 	if structID == "" && direct {
		// 		return structure
		// 	}
		// 	getter := tableGetSliceRValFromPointer(structData).Interface().(zui.ListViewIDGetter)
		// 	count := v.GetRowCount()
		// 	for i := 0; i < count; i++ {
		// 		if getter.GetID(i) == structID {
		// 			return v.GetRowData(i)
		// 		}
		// 	}
		// 	zlog.Fatal(nil, "no row for struct id in table:", direct, structID)
		// 	return nil
		// }
	}
	options := zreflect.Options{UnnestAnonymous: true, MakeSliceElementIfNone: true}
	froot, err := zreflect.ItterateStruct(structure, options)
	if err != nil {
		panic(err)
	}

	for i, item := range froot.Children {
		var f Field
		if f.makeFromReflectItem(structure, item, i) {
			v.fields = append(v.fields, f)
		}
	}
	if header {
		v.Header = zui.HeaderViewNew(name + ".header")
		v.Add(zgeo.Left|zgeo.Top|zgeo.HorExpand, v.Header)
		v.Header.SortingPressed = func() {
			val := tableGetSliceRValFromPointer(structData)
			nval := reflect.MakeSlice(val.Type(), val.Len(), val.Len())
			reflect.Copy(nval, val)
			nslice := nval.Interface()
			slice := val.Interface()
			SortSliceWithFields(nslice, v.fields, v.Header.SortOrder)
			val.Set(nval)
			v.UpdateWithOldNewSlice(slice, nslice)
		}
	}
	v.List = zui.ListViewNew(v.ObjectName() + ".list")
	v.List.SetMinSize(zgeo.Size{50, 50})
	v.List.RowColors = []zgeo.Color{zgeo.ColorNewGray(0.97, 1), zgeo.ColorNewGray(0.85, 1)}
	v.List.HandleScrolledToRows = func(y float64, first, last int) {
		// v.ArrangeChildren(nil)
	}
	v.List.HoverColor = DefaultTableRowHoverColor

	v.Add(zgeo.Left|zgeo.Top|zgeo.Expand, v.List)
	if !rval.IsNil() {
		v.List.RowUpdater = func(i int, edited bool) {
			v.FlushDataToRow(i)
			// code below is EXACTLY flush???
			// rowStack, _ := v.List.GetVisibleRowViewFromIndex(i).(*zui.StackView)
			// if rowStack != nil {
			// 	rowStruct := v.GetRowData(i)
			// 	//				v.handleUpdate(edited, i)
			// 	updateStack(&v.fieldOwner, rowStack, rowStruct)
			// }
		}
	}
	v.GetRowHeight = func(i int) float64 { // default height
		return 50
	}
	v.List.CreateRow = func(rowSize zgeo.Size, i int) zui.View {
		// start := time.Now()
		getter := tableGetSliceRValFromPointer(structData).Interface().(zui.ListViewIDGetter)
		rowID := getter.GetID(i)
		r := v.createRow(rowSize, rowID, i)
		return r
	}
	v.List.GetRowHeight = func(i int) float64 {
		return v.GetRowHeight(i)
	}
	v.GetRowHeight = func(i int) float64 {
		return v.DefaultHeight
	}
	v.List.GetRowCount = func() int {
		return v.GetRowCount()
	}

	// v.handleUpdate = func(edited bool, i int) {
	// 	rowStruct := v.GetRowData(i)
	// 	showError := true
	// 	 zlog.Info("table handleUpdate:", i, edited)
	// 	rowStack, _ := v.List.GetVisibleRowViewFromIndex(i).(*StackView)
	// 	if rowStack != nil {
	// 		FieldsCopyBack(rowStruct, v.fields, rowStack, showError)
	// 	}
	// 	v.List.UpdateRow(i, edited)
	// }
	return v
}

// func (v *TableView) SetRect(rect zgeo.Rect) zui.View {
// 	v.StackView.SetRect(rect)
// 	if v.GetRowCount() > 0 && v.Header != nil {
// 		zlog.Info("TV SetRect fit")
// 		stack := v.List.GetVisibleRowViewFromIndex(0).(*zui.StackView)
// 		v.Header.FitToRowStack(stack, v.ColumnMargin)
// 	}
// 	return v
// }

func (v *TableView) ArrangeChildren(onlyChild *zui.View) {
	v.StackView.ArrangeChildren(onlyChild)
	if v.GetRowCount() > 0 && v.Header != nil {
		first, last := v.List.GetFirstLastVisibleRowIndexes()
		// zlog.Info("TV ArrangeChildren", first, last, zlog.GetCallingStackString())
		for i := first; i <= last; i++ {
			view := v.List.GetVisibleRowViewFromIndex(i)
			if view != nil {
				fv := view.(*FieldView)
				v.Header.FitToRowStack(&fv.StackView, v.ColumnMargin)
			}
		}
	}
}

func (v *TableView) ReadyToShow(beforeWindow bool) {
	if beforeWindow && v.Header != nil {
		headers := makeHeaderFields(v.fields, v.HeaderHeight)
		// zlog.Info("TableView.ReadyToShow:", v.HeaderLongPressed)
		v.Header.Populate(headers)
		v.Header.HeaderPressed = v.HeaderPressed
		v.Header.HeaderLongPressed = v.HeaderLongPressed
		slice := tableGetSliceRValFromPointer(v.structure).Interface()
		var sid string
		getter := slice.(zui.ListViewIDGetter)
		if v.List.SelectionIndex() != -1 {
			sid = getter.GetID(v.List.SelectionIndex())
		}
		SortSliceWithFields(slice, v.fields, v.Header.SortOrder)
		if sid != "" {
			count := v.GetRowCount()
			for i := 0; i < count; i++ {
				if getter.GetID(i) == sid {
					v.List.Select(i)
					break
				}
			}
		}
	}
}

func (v *TableView) Reload() {
	v.List.ReloadData()
}

func (v *TableView) SetStructureList(list interface{}) {
	vs := reflect.ValueOf(list)
	v.GetRowCount = func() int {
		return vs.Len()
	}
	v.GetRowData = func(i int) interface{} {
		if vs.Len() != 0 {
			return vs.Index(i).Addr().Interface()
		}
		return nil
	}
}

func (v *TableView) FlashRow() {
}

func (v *TableView) FlushDataToRow(i int) {
	fv, _ := v.List.GetVisibleRowViewFromIndex(i).(*FieldView)
	if fv != nil {
		data := v.GetRowData(i)
		if data != nil {
			fv.SetStructure(data)
			fv.Update()
		}
		// getter := tableGetSliceRValFromPointer(v.structure).Interface().(zui.ListViewIDGetter)
	}
}

func (v *TableView) createRow(rowSize zgeo.Size, rowID string, i int) zui.View {
	// start := time.Now()
	name := "row " + rowID
	// zlog.Info("createRow:", time.Since(start))
	data := v.GetRowData(i)
	// zlog.Info("createRow2:", time.Since(start))
	fv := FieldViewNew(rowID, data, 0)
	fv.Vertical = false
	fv.fields = v.fields
	fv.SetSpacing(0)
	fv.SetCanFocus(true)
	fv.SetMargin(zgeo.RectMake(v.RowInset, 0, -v.RowInset, 0))
	//	rowStruct := v.GetRowData(i)
	useWidth := true //(v.Header != nil)
	// zlog.Info("createRow4:", time.Since(start))
	fv.buildStack(name, zgeo.Center, zgeo.Size{v.ColumnMargin, 0}, useWidth, v.RowInset)
	// zlog.Info("createRow5:", time.Since(start))
	// edited := false
	// v.handleUpdate(edited, i)
	fv.Update()
	// zlog.Info("createRow6:", time.Since(start))
	return fv
}

func makeHeaderFields(fields []Field, height float64) []zui.Header {
	var headers []zui.Header
	for _, f := range fields {
		var h zui.Header
		h.Height = f.Height
		h.ID = f.ID
		if f.Height == 0 {
			h.Height = height - 6
		}
		if f.Kind == zreflect.KindString && f.Enum == "" {
			h.Align = zgeo.HorExpand
		}
		if f.Flags&flagHasHeaderImage != 0 {
			h.ImageSize = f.Size
			if h.ImageSize.IsNull() {
				h.ImageSize = zgeo.SizeBoth(height - 8)
			}
			h.ImagePath = f.FixedPath
			// zlog.Info("makeHeaderFields:", f.Name, h.ImageSize, h.ImagePath, f)
		}
		if f.Flags&(flagHasHeaderImage|flagNoTitle) == 0 {
			h.Title = f.Title
			if h.Title == "" {
				h.Title = f.Name
			}
		}
		if f.Tooltip != "" && !strings.HasPrefix(f.Tooltip, ".") {
			h.Tip = f.Tooltip
		}
		h.Align |= zgeo.Left | zgeo.VertCenter
		h.SortSmallFirst = f.SortSmallFirst
		h.SortPriority = f.SortPriority
		headers = append(headers, h)
	}
	return headers
}

func (v *TableView) UpdateWithOldNewSlice(oldSlice, newSlice interface{}) {
	oldGetter := oldSlice.(zui.ListViewIDGetter)
	newGetter := newSlice.(zui.ListViewIDGetter)
	// zlog.Info("SLICE5:", oldGetter.GetID(5))
	if v.Header != nil {
		// zlog.Info("sort table")
		SortSliceWithFields(newSlice, v.fields, v.Header.SortOrder)
	}
	v.List.UpdateWithOldNewSlice(oldGetter, newGetter)
}
