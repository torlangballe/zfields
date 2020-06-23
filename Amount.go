package zfields

import (
	"github.com/torlangballe/zui"
	"github.com/torlangballe/zutil/zgeo"
	"github.com/torlangballe/zutil/zlog"
)

// AmountBarValue is Is an amount from 0-1, -1
// It is usd to create a zui.AmountView, since it implements the
// zfield.ActionFieldHandler interface and hooks into it to setup, create and update the view
type AmountBarValue float64

func (a AmountBarValue) HandleAction(f *Field, action ActionType, view *zui.View) bool {
	switch action {
	case EditedAction, DataChangedAction:
		zlog.Assert(view != nil && *view != nil)
		progress := (*view).(*zui.AmountView)
		progress.SetValue(float64(a))

	case CreateAction:
		min := f.MinWidth
		if min == 0 {
			min = 100
		}
		progress := zui.AmountViewBarNew(min)
		if len(f.Colors) != 0 {
			col := zgeo.ColorFromString(f.Colors[0])
			if col.Valid {
				progress.SetColor(col)
			}
		}
		*view = progress
		return true
	}
	return false
}

// AmountCirclesValue is an amount from 0-1, -1
// It is used to create a zui.AmountView circle, since it implements the
// zfield.ActionFieldHandler interface and hooks into it to setup, create and update the view
type AmountCircleValue float64

func createAmountView(f *Field) zui.View {
	v := zui.AmountViewCircleNew()
	v.SetColor(zgeo.ColorNew(0, 0.8, 0, 1))
	for i, n := range []float64{0, 70, 90} {
		if i < len(f.Colors) {
			v.ColorsFromValue[n] = zgeo.ColorFromString(f.Colors[i])
		}
	}
	return v
}

func (a AmountCircleValue) HandleAction(f *Field, action ActionType, view *zui.View) bool {
	const cpuSpace = 1
	switch action {
	case CreateAction:
		*view = createAmountView(f)
		return true

	case SetupAction:
		f.MinWidth = 24
		return true

	case EditedAction, DataChangedAction:
		zlog.Assert(view != nil && *view != nil)
		av := (*view).(*zui.AmountView)
		av.SetValue(float64(a))
		return true
	}
	return false
}
