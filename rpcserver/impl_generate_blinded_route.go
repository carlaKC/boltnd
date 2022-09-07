package rpcserver

import (
	"context"
	"errors"
	"math"

	"github.com/carlakc/boltnd/offersrpc"
	"github.com/lightningnetwork/lnd/lnwire"
	lndwire "github.com/lightningnetwork/lnd/lnwire"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ErrFeatureOverflow is returned if a feature will overflow uint16.
var ErrFeatureOverflow = errors.New("feature exceeds maximum value")

// GenerateBlindedRoute generates a blinded route to our node.
func (s *Server) GenerateBlindedRoute(ctx context.Context,
	req *offersrpc.GenerateBlindedRouteRequest) (
	*offersrpc.GenerateBlindedRouteResponse, error) {

	log.Debugf("GenerateBlindedRoute: %+v", req)

	if err := s.waitForReady(ctx); err != nil {
		return nil, err
	}

	features, err := parseGenerateBlindedRouteRequest(req)
	if err != nil {
		return nil, err
	}

	route, err := s.routeGenerator.BlindedRoute(ctx, features)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &offersrpc.GenerateBlindedRouteResponse{
		Route: composeBlindedRoute(route),
	}, nil
}

// parseGenerateBlindedRouteRequest parses and validates the parameters provided
// by GenerateBlindedRoute.
//
// All errors returned *must* include a grpc status code.
func parseGenerateBlindedRouteRequest(req *offersrpc.GenerateBlindedRouteRequest) (
	[]lndwire.FeatureBit, error) {

	features := make([]lndwire.FeatureBit, len(req.Features))
	for i, feature := range req.Features {
		if feature > math.MaxUint16 {
			return nil, status.Errorf(codes.InvalidArgument,
				"%v: %v", ErrFeatureOverflow, feature)
		}

		features[i] = lnwire.FeatureBit(feature)
	}

	return features, nil
}
