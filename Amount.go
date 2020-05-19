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
	// zlog.Info("ABV Handle Action:", f.Name, action)
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
		if f.Color != "" {
			col := zgeo.ColorFromString(f.Color)
			if col.Valid {
				progress.SetColor(col)
			}
		}
		*view = progress
		return true
	}
	return false
}

// AmountCirclesValue is a slice of amounts from 0-1, -1
// It is usd to create a stack of zui.AmountView circles, since it implements the
// zfield.ActionFieldHandler interface and hooks into it to setup, create and update the view
// TODO: Make on for a cicle amout circle using just a float
type AmountCirclesValue []float64

func createCPUAmountView() zui.View {
	v := zui.AmountViewCircleNew()
	v.SetColor(zgeo.ColorNew(0, 0.8, 0, 1))
	v.ColorsFromValue[70] = zgeo.ColorOrange
	v.ColorsFromValue[90] = zgeo.ColorRed
	return v
}

func (a AmountCirclesValue) HandleAction(f *Field, action ActionType, view *zui.View) bool {
	const cpuSpace = 1
	count := len(a)
	switch action {
	case CreateAction:
		// zlog.Info("Create CPU View", count)
		if count == 0 {
			return true
		}
		stack := zui.StackViewHor("cpu-stack")
		stack.SetSpacing(cpuSpace)
		for count > 0 {
			stack.Add(zgeo.Left|zgeo.VertCenter, createCPUAmountView())
			count--
		}
		*view = stack
		return true

	case SetupAction:
		f.MinWidth = float64(count*24 + (count-1)*cpuSpace)
		return true

	case EditedAction, DataChangedAction:
		zlog.Assert(view != nil && *view != nil)
		stack := (*view).(*zui.StackView)
		for i, v := range stack.GetChildren() {
			if i >= len(a) {
				continue
			}
			av := v.(*zui.AmountView)
			p := float64(a[i])
			av.SetValue(p)
		}
		return true
	}
	return false
}
