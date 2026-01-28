package connectors

import (
	"context"
	"fmt"
	"time"

	"encoding/json"

	"google.golang.org/protobuf/types/known/structpb"

	pb "github.com/xela07ax/spaceai-infra-prototype/pkg/api/connector/v1"
)

type GRPCAdapter struct {
	client pb.ConnectorServiceClient
}

// NewGRPCAdapter создает экземпляр адаптера
func NewGRPCAdapter(client pb.ConnectorServiceClient) *GRPCAdapter {
	return &GRPCAdapter{
		client: client,
	}
}

// Call реализует интерфейс ExecutionProvider
func (a *GRPCAdapter) Call(ctx context.Context, capID string, payload []byte) ([]byte, error) {
	// 1. Конвертируем JSON-байты в Protobuf Struct
	var m map[string]interface{}
	if err := json.Unmarshal(payload, &m); err != nil {
		return nil, fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	protoStruct, err := structpb.NewStruct(m)
	if err != nil {
		return nil, fmt.Errorf("failed to create proto struct: %w", err)
	}

	// 2. Устанавливаем защитный таймаут на уровне вызова
	// Даже если ReliabilityWrapper имеет свой, адаптер должен иметь свой предел
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	// 3. Выполняем gRPC вызов к коннектору
	resp, err := a.client.Execute(ctx, &pb.ExecuteRequest{
		CapabilityId: capID,
		Payload:      protoStruct,
		Metadata:     map[string]string{"source": "uag-engine"},
	})

	if err != nil {
		// Здесь можно добавить логику обработки status.Code(err)
		return nil, fmt.Errorf("connector call failed: %v", err)
	}

	// 4. Проверяем статус внутри ответа
	if resp.StatusCode != 0 {
		return nil, fmt.Errorf("connector returned error [%d]: %s", resp.StatusCode, resp.ErrorMessage)
	}

	// 5. Маршалим результат обратно в JSON для шлюза
	resultBytes, err := json.Marshal(resp.Result.AsMap())
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	return resultBytes, nil
}
