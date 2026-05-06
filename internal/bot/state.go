package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"vk_neuro_bot/internal/bot/flows"
)

const stateTTL = 24 * time.Hour

type StateManager struct {
	rdb *redis.Client
}

func NewStateManager(rdb *redis.Client) *StateManager {
	return &StateManager{rdb: rdb}
}

// Get реализует flows.StateMgr.
func (s *StateManager) Get(ctx context.Context, vkID int64) (*flows.State, error) {
	data, err := s.rdb.Get(ctx, stateKey(vkID)).Bytes()
	if err == redis.Nil {
		return &flows.State{Step: flows.StepWelcome}, nil
	}
	if err != nil {
		return nil, err
	}
	var st flows.State
	if err := json.Unmarshal(data, &st); err != nil {
		return &flows.State{Step: flows.StepWelcome}, nil
	}
	return &st, nil
}

// Set реализует flows.StateMgr.
func (s *StateManager) Set(ctx context.Context, vkID int64, st *flows.State) error {
	data, err := json.Marshal(st)
	if err != nil {
		return err
	}
	return s.rdb.Set(ctx, stateKey(vkID), data, stateTTL).Err()
}

// SetStep реализует flows.StateMgr.
func (s *StateManager) SetStep(ctx context.Context, vkID int64, step string) error {
	return s.Set(ctx, vkID, &flows.State{Step: step})
}

// Reset реализует flows.StateMgr.
func (s *StateManager) Reset(ctx context.Context, vkID int64) error {
	return s.rdb.Del(ctx, stateKey(vkID)).Err()
}

func stateKey(vkID int64) string {
	return fmt.Sprintf("state:%d", vkID)
}
