package apiserver

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	awscreds "github.com/aws/aws-sdk-go-v2/credentials"
	ec2api "github.com/aws/aws-sdk-go-v2/service/ec2"
	ecsapi "github.com/aws/aws-sdk-go-v2/service/ecs"
	elbv2api "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	rdsapi "github.com/aws/aws-sdk-go-v2/service/rds"
	stsapi "github.com/aws/aws-sdk-go-v2/service/sts"
	internalaws "github.com/gocools-LLC/arch.gocools/internal/aws"
	awsdiscovery "github.com/gocools-LLC/arch.gocools/internal/discovery/aws"
	"github.com/gocools-LLC/arch.gocools/internal/discovery/model"
	"github.com/gocools-LLC/arch.gocools/internal/drift"
	"github.com/gocools-LLC/arch.gocools/internal/graph"
	"github.com/gocools-LLC/arch.gocools/internal/stack/lifecycle"
)

type GraphQueryService interface {
	Query(ctx context.Context, query graph.Query) (graph.Graph, error)
}

type StackLifecycleService interface {
	Apply(request lifecycle.Request) (lifecycle.Result, error)
}

type Config struct {
	Addr         string
	Version      string
	Logger       *slog.Logger
	GraphService GraphQueryService
	StackService StackLifecycleService
}

type statusResponse struct {
	Service string `json:"service"`
	Status  string `json:"status"`
	Version string `json:"version"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func New(cfg Config) *http.Server {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	addr := cfg.Addr
	if addr == "" {
		addr = ":8081"
	}

	version := cfg.Version
	if version == "" {
		version = "dev"
	}

	graphService := cfg.GraphService
	if graphService == nil {
		graphService = graph.NewService(graph.NewStaticResourceProvider(defaultResources()))
	}
	stackService := cfg.StackService
	if stackService == nil {
		stackService = lifecycle.NewService()
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", statusHandler(version, "ok"))
	mux.HandleFunc("/readyz", statusHandler(version, "ready"))
	mux.HandleFunc("/api/v1/graph", graphHandler(graphService))
	mux.HandleFunc("/api/v1/discovery/aws/graph", awsGraphHandler())
	mux.HandleFunc("/api/v1/graph/diff", graphDiffHandler())
	mux.HandleFunc("/api/v1/drift", driftHandler())
	mux.HandleFunc("/api/v1/stacks/operations", stackOperationHandler(stackService))

	return &http.Server{
		Addr:              addr,
		Handler:           requestLogMiddleware(logger, mux),
		ReadHeaderTimeout: 5 * time.Second,
	}
}

func statusHandler(version string, status string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
			return
		}

		writeJSON(w, http.StatusOK, statusResponse{
			Service: "arch",
			Status:  status,
			Version: version,
		})
	}
}

func graphHandler(service GraphQueryService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
			return
		}

		result, err := service.Query(r.Context(), graph.Query{
			StackID:     r.URL.Query().Get("stack_id"),
			Environment: r.URL.Query().Get("environment"),
		})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
			return
		}

		writeJSON(w, http.StatusOK, result)
	}
}

func stackOperationHandler(service StackLifecycleService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
			return
		}

		var request lifecycle.Request
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid json payload"})
			return
		}

		result, err := service.Apply(request)
		if err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, context.Canceled) {
				status = http.StatusRequestTimeout
			}
			writeJSON(w, status, errorResponse{Error: err.Error()})
			return
		}

		writeJSON(w, http.StatusOK, result)
	}
}

type driftRequest struct {
	Desired            []model.Resource `json:"desired"`
	Actual             []model.Resource `json:"actual"`
	IgnoredMetadataKey []string         `json:"ignored_metadata_keys,omitempty"`
}

type graphDiffRequest struct {
	Before      graph.Graph `json:"before"`
	After       graph.Graph `json:"after"`
	StackID     string      `json:"stack_id,omitempty"`
	Environment string      `json:"environment,omitempty"`
}

type awsGraphRequest struct {
	Region          string `json:"region"`
	AccessKeyID     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"`
	SessionToken    string `json:"session_token,omitempty"`
	RoleARN         string `json:"role_arn,omitempty"`
	SessionName     string `json:"session_name,omitempty"`
	ExternalID      string `json:"external_id,omitempty"`
	StackID         string `json:"stack_id,omitempty"`
	Environment     string `json:"environment,omitempty"`
	ValidateOnStart bool   `json:"validate_on_start,omitempty"`
}

type awsIdentity struct {
	AccountID string `json:"account_id,omitempty"`
	ARN       string `json:"arn,omitempty"`
	UserID    string `json:"user_id,omitempty"`
}

type awsGraphResponse struct {
	Connected bool        `json:"connected"`
	Provider  string      `json:"provider"`
	Region    string      `json:"region"`
	Identity  awsIdentity `json:"identity"`
	Graph     graph.Graph `json:"graph"`
}

func driftHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
			return
		}

		var request driftRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid json payload"})
			return
		}

		report := drift.BuildReport(request.Desired, request.Actual, drift.Config{
			IgnoredMetadataKeys: request.IgnoredMetadataKey,
		})
		writeJSON(w, http.StatusOK, report)
	}
}

func graphDiffHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
			return
		}

		var request graphDiffRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid json payload"})
			return
		}

		report := graph.DiffGraphs(request.Before, request.After, graph.Query{
			StackID:     request.StackID,
			Environment: request.Environment,
		})

		writeJSON(w, http.StatusOK, report)
	}
}

func awsGraphHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
			return
		}

		var request awsGraphRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid json payload"})
			return
		}

		request.Region = strings.TrimSpace(request.Region)
		request.AccessKeyID = strings.TrimSpace(request.AccessKeyID)
		request.SecretAccessKey = strings.TrimSpace(request.SecretAccessKey)
		request.SessionToken = strings.TrimSpace(request.SessionToken)
		request.RoleARN = strings.TrimSpace(request.RoleARN)
		request.SessionName = strings.TrimSpace(request.SessionName)
		request.ExternalID = strings.TrimSpace(request.ExternalID)
		request.StackID = strings.TrimSpace(request.StackID)
		request.Environment = strings.TrimSpace(request.Environment)

		if request.Region == "" {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "region is required"})
			return
		}
		if request.AccessKeyID == "" {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "access_key_id is required"})
			return
		}
		if request.SecretAccessKey == "" {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "secret_access_key is required"})
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
		defer cancel()

		session := internalaws.SessionConfig{
			Region:      request.Region,
			RoleARN:     request.RoleARN,
			SessionName: request.SessionName,
			ExternalID:  request.ExternalID,
		}

		loadOptions := []func(*awsconfig.LoadOptions) error{
			awsconfig.WithRegion(request.Region),
			awsconfig.WithCredentialsProvider(
				awscreds.NewStaticCredentialsProvider(request.AccessKeyID, request.SecretAccessKey, request.SessionToken),
			),
		}

		if request.ValidateOnStart {
			if err := internalaws.ValidateCredentials(ctx, session, loadOptions...); err != nil {
				writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
				return
			}
		}

		awsCfg, err := internalaws.LoadConfig(ctx, session, loadOptions...)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
			return
		}

		identityOutput, err := stsapi.NewFromConfig(awsCfg).GetCallerIdentity(ctx, &stsapi.GetCallerIdentityInput{})
		if err != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
			return
		}

		discoverer := awsdiscovery.NewDiscoverer(awsdiscovery.Clients{
			EC2:   ec2api.NewFromConfig(awsCfg),
			ECS:   ecsapi.NewFromConfig(awsCfg),
			ELBV2: elbv2api.NewFromConfig(awsCfg),
			RDS:   rdsapi.NewFromConfig(awsCfg),
		}, awsdiscovery.Config{
			Region: request.Region,
		})

		resources, err := discoverer.DiscoverAll(ctx)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
			return
		}

		discoveredGraph := graph.FromResources(resources, time.Now()).Filter(graph.Query{
			StackID:     request.StackID,
			Environment: request.Environment,
		})

		response := awsGraphResponse{
			Connected: true,
			Provider:  "aws",
			Region:    request.Region,
			Identity: awsIdentity{
				AccountID: stringPointerValue(identityOutput.Account),
				ARN:       stringPointerValue(identityOutput.Arn),
				UserID:    stringPointerValue(identityOutput.UserId),
			},
			Graph: discoveredGraph,
		}

		writeJSON(w, http.StatusOK, response)
	}
}

func stringPointerValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func writeJSON(w http.ResponseWriter, code int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(payload)
}

func requestLogMiddleware(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &responseRecorder{ResponseWriter: w}
		next.ServeHTTP(rec, r)

		status := rec.statusCode
		if status == 0 {
			status = http.StatusOK
		}

		logger.Info(
			"http_request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", status,
			"duration_ms", time.Since(start).Milliseconds(),
			"remote_addr", r.RemoteAddr,
		)
	})
}

type responseRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (r *responseRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

func (r *responseRecorder) Write(p []byte) (int, error) {
	if r.statusCode == 0 {
		r.statusCode = http.StatusOK
	}
	return r.ResponseWriter.Write(p)
}

func defaultResources() []model.Resource {
	return []model.Resource{
		{
			ID:       "i-dev-1",
			Type:     "aws.ec2.instance",
			Provider: "aws",
			Region:   "us-east-1",
			Name:     "dev-instance",
			State:    "running",
			Tags: map[string]string{
				"gocools:stack-id":    "dev-stack",
				"gocools:environment": "dev",
				"gocools:owner":       "platform-team",
			},
		},
		{
			ID:       "i-prod-1",
			Type:     "aws.ec2.instance",
			Provider: "aws",
			Region:   "us-east-1",
			Name:     "prod-instance",
			State:    "running",
			Tags: map[string]string{
				"gocools:stack-id":    "prod-stack",
				"gocools:environment": "prod",
				"gocools:owner":       "platform-team",
			},
		},
	}
}
