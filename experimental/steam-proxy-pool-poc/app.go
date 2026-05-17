package main

import (
	"context"
	"strings"
	"time"
)

type App struct {
	ctx      context.Context
	detector *Detector
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.detector = NewDetector(DefaultProbeConfig())
}

func (a *App) RunDiagnosis(manualProxy string) (*DiagnosisReport, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Second)
	defer cancel()
	return a.detector.RunDiagnosis(ctx, strings.TrimSpace(manualProxy))
}

func (a *App) ScanLocalProxies() ([]ProbeResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 55*time.Second)
	defer cancel()
	return a.detector.ScanLocalCandidates(ctx), nil
}

func (a *App) TestManualProxy(manualProxy string) (*ProbeResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	result := a.detector.TestManualProxy(ctx, strings.TrimSpace(manualProxy))
	return &result, nil
}
