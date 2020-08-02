package zfields

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/torlangballe/zui"
	"github.com/torlangballe/zutil/zbool"
	"github.com/torlangballe/zutil/zdict"
	"github.com/torlangballe/zutil/zfloat"
	"github.com/torlangballe/zutil/zgeo"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zreflect"
	"github.com/torlangballe/zutil/zstr"
)

// type fieldType int

type ActionType string

const (
	DataChangedAction     ActionType = "changed"     // called when value changed, typically programatically or edited
	EditedAction          ActionType = "edited"      // called when value edited by user, DataChangedAction will also be called
	SetupFieldAction      ActionType = "setup"       // called when a field is being set up from a struct, view will be nil
	PressedAction         ActionType = "pressed"     // called when view is pressed, view is valid
	LongPressedAction     ActionType = "longpressed" // called when view is long-pressed, view is valid
	NewStructAction       ActionType = "newstruct"   // called when new stucture is created, for initializing. View may  be nil
	CreateFieldViewAction ActionType = "createview"  // called to create view, view is pointer to view and is returned in it
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
	flagIsNamedSelection
	flagIsTabGroup
	flagIsStringer
	flagIsPassword
	flagExpandFromMinSize
)

const (
	flagTimeFlags = flagHasSeconds | flagHasMinutes | flagHasHours
	flagDateFlags = flagHasDays | flagHasMonths | flagHasYears
)

type Field struct {
	Index     int
	ID        string
	Name      string
	FieldName string
	Title     string // name of item in row, and header if no title
	// Width         float64
	MaxWidth      float64
	MinWidth      float64
	Kind          zreflect.TypeKind
	Vertical      zbool.BoolInd
	Alignment     zgeo.Alignment
	Justify       zgeo.Alignment
	Format        string
	Colors        []string
	FixedPath     string
	Height        float64
	Enum          string
	LocalEnum     string
	Size          zgeo.Size
	Flags         int
	Tooltip       string
	UpdateSecs    float64
	LabelizeWidth float64
	LocalEnable   string
	FontSize      float64
	FontName      string
	FontStyle     zui.FontStyle
	Spacing       float64
	Placeholder   string
}

type ActionHandler interface {
	HandleAction(f *Field, action ActionType, view *zui.View) bool
}

type ActionFieldHandler interface {
	HandleFieldAction(f *Field, action ActionType, view *zui.View) bool
}

func (f Field) IsStatic() bool {
	return f.Flags&flagIsStatic != 0
}

func (f *Field) SetFont(view zui.View, from *zui.Font) {
	to := view.(zui.TextLayoutOwner)
	size := f.FontSize
	if size == 0 {
		if from != nil {
			size = from.Size
		} else {
			size = zui.FontDefaultSize
		}
	}
	style := f.FontStyle
	if from != nil {
		style = from.Style
	}
	var font *zui.Font
	if f.FontName != "" {
		font = zui.FontNew(f.FontName, size, style)
	} else if from != nil {
		font = new(zui.Font)
		*font = *from
	} else {
		font = zui.FontNice(size, style)
	}
	to.SetFont(font)
}

func findFieldWithIndex(fields *[]Field, index int) *Field {
	for i, f := range *fields {
		if f.Index == index {
			return &(*fields)[i]
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

func findLocalField(children *[]zreflect.Item, name string) *zreflect.Item {
	name = zstr.HeadUntil(name, ".")
	for i, c := range *children {
		if c.FieldName == name {
			return &(*children)[i]
		}
	}
	return nil
}

func (f *Field) makeFromReflectItem(structure interface{}, item zreflect.Item, index int) bool {
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
		case "vertical":
			f.Vertical = zbool.True
		case "horizontal":
			f.Vertical = zbool.False
		case "align":
			if align != zgeo.AlignmentNone {
				f.Alignment = align
			}
			// zlog.Info("ALIGN:", f.Name, val, a)
		case "nosize":
			f.Flags |= flagExpandFromMinSize

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
		case "named-selection":
			f.Flags |= flagIsNamedSelection
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
		case "placeholder":
			if val != "" {
				f.Placeholder = val
			} else {
				f.Placeholder = "$HAS$"
			}
		}

	}
	if f.Flags&flagToClipboard != 0 && f.Tooltip == "" {
		f.Tooltip = "press to copy to Clipboard"
	}
	// zfloat.Maximize(&f.MaxWidth, f.MinWidth)
	zfloat.Minimize(&f.MinWidth, f.MaxWidth)
	if f.Name == "" {
		str := zstr.PadCamelCase(item.FieldName, " ")
		str = zstr.FirstToTitleCase(str)
		f.Name = str
	}
	if f.Placeholder == "$HAS$" {
		f.Placeholder = f.Name
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
	callActionHandlerFunc(structure, f, SetupFieldAction, item.Interface, nil) // need to use v.structure here, since i == -1
	return true
}

var fieldEnums = map[string]zdict.Items{}

func SetEnum(name string, enum zdict.Items) {
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
