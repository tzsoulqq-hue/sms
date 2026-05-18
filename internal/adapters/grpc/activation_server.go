package grpcadapter

import (
	"context"

	smsv1 "github.com/byte-v-forge/contracts-go/byte/v/forge/contracts/sms/v1"
	"github.com/byte-v-forge/sms/internal/app"
	"github.com/byte-v-forge/sms/internal/core"
)

type ActivationServer struct {
	smsv1.UnimplementedSmsActivationServiceServer
	service *app.ActivationService
}

func NewActivationServer(service *app.ActivationService) *ActivationServer {
	return &ActivationServer{service: service}
}

func (s *ActivationServer) AcquireNumber(ctx context.Context, request *smsv1.AcquireNumberRequest) (*smsv1.AcquireNumberResponse, error) {
	activation, err := s.service.AcquireNumber(ctx, core.AcquireNumberCommand{
		RequestID:     request.GetRequestId(),
		Target:        fromProtoTarget(request.GetTarget()),
		LeaseDuration: protoDuration(request.GetLeaseDuration()),
		Labels:        request.GetLabels(),
	})
	if err != nil {
		return &smsv1.AcquireNumberResponse{Error: toProtoError(err)}, nil
	}
	return &smsv1.AcquireNumberResponse{Activation: toProtoActivation(activation)}, nil
}

func (s *ActivationServer) GetActivation(ctx context.Context, request *smsv1.GetActivationRequest) (*smsv1.GetActivationResponse, error) {
	activation, err := s.service.GetActivation(ctx, request.GetActivationId())
	if err != nil {
		return &smsv1.GetActivationResponse{Error: toProtoError(err)}, nil
	}
	return &smsv1.GetActivationResponse{Activation: toProtoActivation(activation), Code: toProtoCode(activation.Code)}, nil
}

func (s *ActivationServer) WaitForCode(ctx context.Context, request *smsv1.WaitForCodeRequest) (*smsv1.WaitForCodeResponse, error) {
	activation, code, err := s.service.WaitForCode(
		ctx,
		request.GetActivationId(),
		protoDuration(request.GetTimeout()),
		protoDuration(request.GetPollInterval()),
	)
	if err != nil {
		return &smsv1.WaitForCodeResponse{Activation: toProtoActivation(activation), Error: toProtoError(err)}, nil
	}
	return &smsv1.WaitForCodeResponse{Activation: toProtoActivation(activation), Code: toProtoCode(code)}, nil
}

func (s *ActivationServer) MarkMessageSent(ctx context.Context, request *smsv1.MarkMessageSentRequest) (*smsv1.MarkMessageSentResponse, error) {
	activation, err := s.service.MarkMessageSent(ctx, request.GetActivationId(), request.GetRequestId())
	if err != nil {
		return &smsv1.MarkMessageSentResponse{Activation: toProtoActivation(activation), Error: toProtoError(err)}, nil
	}
	return &smsv1.MarkMessageSentResponse{Activation: toProtoActivation(activation)}, nil
}

func (s *ActivationServer) RequestAdditionalCode(ctx context.Context, request *smsv1.RequestAdditionalCodeRequest) (*smsv1.RequestAdditionalCodeResponse, error) {
	activation, err := s.service.RequestAdditionalCode(ctx, request.GetActivationId(), request.GetRequestId())
	if err != nil {
		return &smsv1.RequestAdditionalCodeResponse{Activation: toProtoActivation(activation), Error: toProtoError(err)}, nil
	}
	return &smsv1.RequestAdditionalCodeResponse{Activation: toProtoActivation(activation)}, nil
}

func (s *ActivationServer) CompleteActivation(ctx context.Context, request *smsv1.CompleteActivationRequest) (*smsv1.CompleteActivationResponse, error) {
	activation, err := s.service.CompleteActivation(ctx, request.GetActivationId(), request.GetRequestId())
	if err != nil {
		return &smsv1.CompleteActivationResponse{Activation: toProtoActivation(activation), Error: toProtoError(err)}, nil
	}
	return &smsv1.CompleteActivationResponse{Activation: toProtoActivation(activation)}, nil
}

func (s *ActivationServer) CancelActivation(ctx context.Context, request *smsv1.CancelActivationRequest) (*smsv1.CancelActivationResponse, error) {
	activation, err := s.service.CancelActivation(ctx, request.GetActivationId(), request.GetRequestId())
	if err != nil {
		return &smsv1.CancelActivationResponse{Activation: toProtoActivation(activation), Error: toProtoError(err)}, nil
	}
	return &smsv1.CancelActivationResponse{Activation: toProtoActivation(activation)}, nil
}
