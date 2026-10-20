package input

// MouseButton identifies a logical mouse button, independent of any backend
// encoding. Each backend maps it to its own representation.
type MouseButton uint8

const (
	MouseButtonLeft MouseButton = iota + 1
	MouseButtonRight
	MouseButtonMiddle
	MouseButtonX1
	MouseButtonX2
)

// MouseEventKind selects which payload a MouseEvent carries.
type MouseEventKind uint8

const (
	MouseEventMoveRel MouseEventKind = iota + 1
	MouseEventMoveAbs
	MouseEventButton
	MouseEventWheel
	MouseEventHWheel
)

// MouseEvent is one mouse injection operation in a backend-neutral form.
type MouseEvent struct {
	Kind MouseEventKind

	// DX/DY is the relative movement delta in pixels for MouseEventMoveRel.
	DX, DY int32

	// X/Y is the absolute position for MouseEventMoveAbs, normalized to
	// 0..10000 (matching the gRPC MouseEvent ABS_MOVE convention). Each backend
	// scales it to its own absolute range (e.g. FakerInput's 0..32767).
	X, Y int32

	// Button and IsPressed describe MouseEventButton.
	Button    MouseButton
	IsPressed bool

	// Delta is the wheel detent count for MouseEventWheel / MouseEventHWheel.
	Delta int32
}
