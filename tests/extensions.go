package tests

import (
	"fmt"

	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/any"
	"github.com/golang/protobuf/ptypes/wrappers"
	"open-match.dev/open-match/pkg/pb"
)

const (
	playersPerMatch = 3
	openSlotsKey    = "openSlots"
)

func getOpenSlots(b *pb.Backfill) (int32, error) {
	if b == nil {
		return 0, fmt.Errorf("expected backfill is not nil")
	}
	if b.Extensions != nil {
		if any, ok := b.Extensions[openSlotsKey]; ok {
			var val wrappers.Int32Value
			err := ptypes.UnmarshalAny(any, &val)
			if err != nil {
				return 0, err
			}

			return val.Value, nil
		}
	}
	return 0, fmt.Errorf("failed to get openSlots extension (key not found)")
}

func setOpenSlots(b *pb.Backfill, val int32) error {
	if b.Extensions == nil {
		b.Extensions = make(map[string]*any.Any)
	}
	any, err := ptypes.MarshalAny(&wrappers.Int32Value{Value: val})
	if err != nil {
		return err
	}
	b.Extensions[openSlotsKey] = any
	return nil
}
