// Code generated by 'yaegi extract github.com/opennox/libs/wall'. DO NOT EDIT.

package imports

import (
	"go/constant"
	"go/token"
	"reflect"

	"github.com/opennox/libs/wall"
)

func init() {
	Symbols["github.com/opennox/libs/wall/wall"] = map[string]reflect.Value{
		// function, constant and variable definitions
		"Flag1":         reflect.ValueOf(wall.Flag1),
		"Flag8":         reflect.ValueOf(wall.Flag8),
		"FlagBreakable": reflect.ValueOf(wall.FlagBreakable),
		"FlagBroken":    reflect.ValueOf(wall.FlagBroken),
		"FlagDoor":      reflect.ValueOf(wall.FlagDoor),
		"FlagFront":     reflect.ValueOf(wall.FlagFront),
		"FlagSecret":    reflect.ValueOf(wall.FlagSecret),
		"FlagWindow":    reflect.ValueOf(wall.FlagWindow),
		"GridStep":      reflect.ValueOf(constant.MakeFromLiteral("23", token.INT, 0)),
		"GridToPos":     reflect.ValueOf(wall.GridToPos),
		"PosToGrid":     reflect.ValueOf(wall.PosToGrid),

		// type definitions
		"Flags": reflect.ValueOf((*wall.Flags)(nil)),
	}
}
