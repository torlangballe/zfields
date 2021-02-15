package zfields

// AmountBarValue is Is an amount from 0-1, -1
// It is usd to create a zui.AmountView, since it implements the
// zfield.ActionFieldHandler interface and hooks into it to setup, create and update the view
type AmountBarValue float64

// AmountCirclesValue is an amount from 0-1, -1
// It is used to create a zui.AmountView circle, since it implements the
// zfield.ActionFieldHandler interface and hooks into it to setup, create and update the view
type AmountCircleValue float64
