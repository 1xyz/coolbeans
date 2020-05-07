package server

import (
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	healthV1 "google.golang.org/grpc/health/grpc_health_v1"
	"time"
)

type ServiceReadiness interface {
	// Ready indicates if this service is ready to accept traffic
	Ready() bool
}

type HealthCheckServer struct {
	healthV1.UnimplementedHealthServer
	tickDuration     time.Duration
	shutdownCh       chan bool
	serviceReadiness ServiceReadiness
}

func (h *HealthCheckServer) Check(ctx context.Context,
	req *healthV1.HealthCheckRequest) (*healthV1.HealthCheckResponse, error) {
	s := healthV1.HealthCheckResponse_UNKNOWN
	if h.serviceReadiness == nil {
		s = healthV1.HealthCheckResponse_UNKNOWN
	} else if h.serviceReadiness.Ready() {
		s = healthV1.HealthCheckResponse_SERVING
	} else {
		s = healthV1.HealthCheckResponse_NOT_SERVING
	}

	log.Debugf("check: req svc: %v status= %v", req.Service, s)

	return &healthV1.HealthCheckResponse{
		Status: s,
	}, nil
}

func (h *HealthCheckServer) Watch(req *healthV1.HealthCheckRequest, stream healthV1.Health_WatchServer) error {
	ticker := time.NewTicker(h.tickDuration)
	defer ticker.Stop()

	for {
		select {
		case t := <-ticker.C:
			resp, err := h.Check(stream.Context(), req)
			if err != nil {
				return err
			}

			log.Debugf("watch: sending resp at %v resp=%v", t, resp)
			stream.Send(resp)

		case <-h.shutdownCh:
			log.Debugf("watch: shutdown signaled")
			return nil
		}
	}
}

func NewHealthCheckServer(s ServiceReadiness) *HealthCheckServer {
	return &HealthCheckServer{
		tickDuration:     time.Second,
		shutdownCh:       make(chan bool),
		serviceReadiness: s,
	}
}
