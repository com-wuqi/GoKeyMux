package engine

import (
	"GoKeyMux/internal/config"
	"GoKeyMux/internal/input"
)

// Aliases keep the engine signatures and tests concise now that the input
// layer lives in internal/input.
type KeyCodes = input.KeyCodes
type MouseButton = input.MouseButton
type mouseEvent = input.MouseEvent
type mouseEventKind = input.MouseEventKind
type NoopDevice = input.NoopDevice

type DriveName = config.DriveName

const (
	DriveMakc                   = config.DriveMakc
	DriveWinputWithWindow       = config.DriveWinputWithWindow
	DriveWinputWithInterception = config.DriveWinputWithInterception
	DriveFakerInput             = config.DriveFakerInput
	DriveNoop                   = config.DriveNoop

	MouseButtonLeft   = input.MouseButtonLeft
	MouseButtonRight  = input.MouseButtonRight
	MouseButtonMiddle = input.MouseButtonMiddle
	MouseButtonX1     = input.MouseButtonX1
	MouseButtonX2     = input.MouseButtonX2

	mouseEventMoveRel = input.MouseEventMoveRel
	mouseEventMoveAbs = input.MouseEventMoveAbs
	mouseEventButton  = input.MouseEventButton
	mouseEventWheel   = input.MouseEventWheel
	mouseEventHWheel  = input.MouseEventHWheel
)
