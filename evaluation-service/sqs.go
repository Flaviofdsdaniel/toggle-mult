package main

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sqs"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// Evento que será enviado para a fila
type EvaluationEvent struct {
	UserID    string    `json:"user_id"`
	FlagName  string    `json:"flag_name"`
	Result    bool      `json:"result"`
	Timestamp time.Time `json:"timestamp"`
}

// sendEvaluationEvent envia um evento para a fila SQS
func (a *App) sendEvaluationEvent(ctx context.Context, userID, flagName string, result bool) {
	// Se a URL da fila não foi configurada, apenas loga localmente e sai.
	if a.SqsSvc == nil || a.SqsQueueURL == "" {
		log.Printf("[SQS_DISABLED] Evento: User '%s', Flag '%s', Result '%t'", userID, flagName, result)
		return
	}

	tracer := otel.Tracer("evaluation-service/sqs")
	ctx, span := tracer.Start(ctx, "send evaluation event", trace.WithSpanKind(trace.SpanKindProducer))
	defer span.End()
	span.SetAttributes(
		attribute.String("messaging.system", "aws_sqs"),
		attribute.String("messaging.destination.name", "ToggleMasterAnalyticsQueue"),
		attribute.String("messaging.operation", "send"),
		attribute.String("feature_flag.name", flagName),
	)

	event := EvaluationEvent{
		UserID:    userID,
		FlagName:  flagName,
		Result:    result,
		Timestamp: time.Now().UTC(),
	}

	body, err := json.Marshal(event)
	if err != nil {
		log.Printf("Erro ao serializar evento SQS: %v", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to serialize evaluation event")
		return
	}

	// Propaga o contexto W3C pelo SQS sem alterar o contrato JSON do evento.
	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)
	messageAttributes := make(map[string]*sqs.MessageAttributeValue)
	for _, key := range []string{"traceparent", "tracestate"} {
		if value := carrier.Get(key); value != "" {
			messageAttributes[key] = &sqs.MessageAttributeValue{
				DataType:    aws.String("String"),
				StringValue: aws.String(value),
			}
		}
	}

	// Envia a mensagem
	_, err = a.SqsSvc.SendMessageWithContext(ctx, &sqs.SendMessageInput{
		MessageBody:       aws.String(string(body)),
		MessageAttributes: messageAttributes,
		QueueUrl:          aws.String(a.SqsQueueURL),
	})

	if err != nil {
		log.Printf("Erro ao enviar mensagem para SQS: %v", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to send evaluation event to SQS")
	} else {
		log.Printf("Evento de avaliação enviado para SQS (Flag: %s)", flagName)
	}
}
