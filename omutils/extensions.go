package omutils

import (
	"fmt"

	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"open-match.dev/open-match/pkg/pb"
)

const (
	PlayersPerMatch = 3
	openSlotsKey    = "openSlots"
)

func GetOpenSlots(b *pb.Backfill) (int32, error) {
	if b == nil {
		return 0, fmt.Errorf("expected backfill is not nil")
	}
	if b.Extensions != nil {
		if any, ok := b.Extensions[openSlotsKey]; ok {
			var val wrapperspb.Int32Value
			if err := any.UnmarshalTo(&val); err != nil {
				return 0, err
			}
			return val.Value, nil
		}
	}
	return 0, fmt.Errorf("failed to get openSlots extension (key not found)")
}

func SetOpenSlots(b *pb.Backfill, val int32) error {
	if b.Extensions == nil {
		b.Extensions = map[string]*anypb.Any{}
	}
	any, err := anypb.New(&wrapperspb.Int32Value{Value: val})
	if err != nil {
		return err
	}
	b.Extensions[openSlotsKey] = any
	return nil
}
