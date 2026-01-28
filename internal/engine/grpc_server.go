package engine

import (
	"context"
	"encoding/json"

	pb "github.com/xela07ax/spaceai-infra-prototype/pkg/api/connector/v1"
	"google.golang.org/protobuf/types/known/structpb"
)

type GRPCGatewayServer struct {
	pb.UnimplementedConnectorServiceServer
	uag *UAGCore
}

func NewGRPCGatewayServer(uag *UAGCore) *GRPCGatewayServer {
	return &GRPCGatewayServer{uag: uag}
}

func (s *GRPCGatewayServer) Execute(ctx context.Context, req *pb.ExecuteRequest) (*pb.ExecuteResponse, error) {
	// 1. Подготавливаем данные (маршалим Struct в JSON байты для ProcessAction)
	payloadBytes, _ := json.Marshal(req.Payload.AsMap())

	// 2. Извлекаем AgentID из метаданных или самого запроса
	// В gRPC метаданные передаются через context
	agentID := req.Metadata["agent_id"]

	// 3. Вызываем единый пайплайн обработки (Тот же, что и для HTTP!)
	respBytes, err := s.uag.ProcessAction(ctx, agentID, req.CapabilityId, payloadBytes)
	if err != nil {
		return &pb.ExecuteResponse{
			StatusCode:   403,
			ErrorMessage: err.Error(),
		}, nil
	}

	// 4. Собираем ответ обратно в Protobuf
	var resultMap map[string]interface{}
	json.Unmarshal(respBytes, &resultMap)
	resultStruct, _ := structpb.NewStruct(resultMap)

	return &pb.ExecuteResponse{
		StatusCode: 0,
		Result:     resultStruct,
	}, nil
}
