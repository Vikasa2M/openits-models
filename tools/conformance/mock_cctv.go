package main

import (
	"context"
	"time"

	cctvv1 "github.com/Vikasa2M/openits-models/pkg/proto/openits/cctv/v1"
	commonv1 "github.com/Vikasa2M/openits-models/pkg/proto/openits/common/v1"
	yangpkg "github.com/Vikasa2M/openits-models/pkg/yang/openits"
	"github.com/Vikasa2M/openits-models/tools/conformance/tests"
)

// collectCctv builds a fully-populated, spec-compliant CCTV/PTZ observation:
// identity + mounting (with co-located devices), PTZ capabilities/position, a
// preset inventory with a recalled preset, a running tour, an encoder stream,
// enclosure environmental readback, and an active fault.
func collectCctv() (*yangpkg.Device, error) {
	dev := &yangpkg.Device{}
	cam := dev.GetOrCreateCamera()

	cfg := cam.GetOrCreateConfig()
	cfg.Id = strPtr("cctv-i35-mm214")
	cfg.Name = strPtr("I-35 NB @ MM 214 PTZ")
	cfg.RoadReference = strPtr("I-35 NB @ MM 214")
	cfg.Latitude = f64Ptr(30.2672)
	cfg.Longitude = f64Ptr(-97.7431)

	st := cam.GetOrCreateState()
	st.Id = strPtr("cctv-i35-mm214")
	st.Name = strPtr("I-35 NB @ MM 214 PTZ")
	st.Latitude = f64Ptr(30.2672)
	st.Longitude = f64Ptr(-97.7431)
	st.Make = strPtr("Axis")
	st.Model = strPtr("Q6215-LE")
	st.Firmware = strPtr("10.12.0")
	st.OperationalStatus = yangpkg.OpenitsCctvTypes_OperationalStatus_online

	// Mounting + the co-located devices an operator verifies against.
	mount := cam.GetOrCreateMounting()
	mount.Structure = yangpkg.OpenitsCctvTypes_MountingStructure_pole
	mount.HeightM = f64Ptr(9.1)
	for _, id := range []string{"asc-i35-mm214", "ess-i35-mm214"} {
		if _, err := mount.NewAssociatedDevice(id, yangpkg.OpenitsTypes_AssociationRole_role_co_located); err != nil {
			panic(err)
		}
	}

	// PTZ: capabilities, live position, presets (with the recalled one), a tour.
	ptz := cam.GetOrCreatePtz()
	caps := ptz.GetOrCreateCapabilities()
	caps.PtzCapable = boolPtr(true)
	caps.MaxPresets = u16Ptr(256)
	caps.PanContinuous = boolPtr(true)

	pst := ptz.GetOrCreateState()
	pst.PanDegrees = f64Ptr(182.0)
	pst.TiltDegrees = f64Ptr(-12.5)
	pst.ZoomPercent = u8Ptr(40)
	pst.Moving = boolPtr(false)

	presets := ptz.GetOrCreatePresets()
	for _, p := range []struct {
		id   uint16
		name string
		pan  float64
	}{{1, "NB approach", 180.0}, {2, "SB approach", 0.0}} {
		pr, err := presets.NewPreset(p.id)
		if err != nil {
			return nil, err
		}
		pr.Name = strPtr(p.name)
		pr.PanDegrees = f64Ptr(p.pan)
		pr.TiltDegrees = f64Ptr(-10.0)
		pr.ZoomPercent = u8Ptr(30)
	}
	presets.Recall = u16Ptr(1) // leafref -> preset 1 (must exist above)
	presets.GetOrCreateState().ActivePreset = u16Ptr(1)

	tours := ptz.GetOrCreateTours()
	tour, err := tours.NewTour(1)
	if err != nil {
		return nil, err
	}
	tour.Name = strPtr("mainline sweep")
	for _, s := range []struct {
		seq    uint16
		preset uint16
	}{{1, 1}, {2, 2}} {
		stop, err := tour.AppendNewStop(s.seq)
		if err != nil {
			return nil, err
		}
		stop.PresetId = u16Ptr(s.preset) // leafref -> a defined preset
		stop.DwellSeconds = u16Ptr(15)
	}
	tours.Run = u16Ptr(1) // leafref -> tour 1
	tourSt := tours.GetOrCreateState()
	tourSt.ActiveTour = u16Ptr(1)
	tourSt.RunState = yangpkg.OpenitsCctvTypes_TourRunState_running
	tourSt.CurrentStop = u16Ptr(1)

	// Encoder stream (device-side status).
	streams := cam.GetOrCreateStreams()
	stream, err := streams.NewStream(0)
	if err != nil {
		return nil, err
	}
	streamCfg := stream.GetOrCreateConfig()
	streamCfg.Codec = yangpkg.OpenitsCctvTypes_VideoCodec_h264
	streamCfg.TargetBitrateKbps = u32Ptr(4000)
	streamSt := stream.GetOrCreateState()
	streamSt.WidthPx = u16Ptr(1920)
	streamSt.HeightPx = u16Ptr(1080)
	streamSt.Codec = yangpkg.OpenitsCctvTypes_VideoCodec_h264
	streamSt.BitrateKbps = u32Ptr(3850)
	streamSt.FrameRate = f64Ptr(30.0)
	streamSt.Health = yangpkg.OpenitsCctvTypes_StreamHealth_ok

	// Enclosure environmental readback.
	env := cam.GetOrCreateEnvironment()
	envCfg := env.GetOrCreateConfig()
	envCfg.Wiper = boolPtr(false)
	envCfg.Washer = boolPtr(false)
	envCfg.HeaterMode = yangpkg.OpenitsCctv_Camera_Environment_Config_HeaterMode_auto
	envSt := env.GetOrCreateState()
	envSt.EnclosureTempC = f64Ptr(34.5)
	envSt.WiperActive = boolPtr(false)
	envSt.HeaterActive = boolPtr(false)
	envSt.BlowerActive = boolPtr(true)

	// One active fault carrying its category on the shared fault taxonomy.
	faults := cam.GetOrCreateFaults()
	fault, err := faults.NewFault("f-focus-01")
	if err != nil {
		return nil, err
	}
	fault.Category = yangpkg.OpenitsCctvTypes_CctvFaultEventKind_cctv_fault_focus
	fault.Severity = yangpkg.OpenitsTypes_FaultSeverity_minor
	fault.Description = strPtr("Focus hunting in low light")
	fault.FirstObserved = strPtr("2026-07-17T02:00:00Z")

	// Control ownership: central control, held by a TMC operator. Commanded
	// control-mode matches the actual (no local override); the holder readback
	// records who holds control and at what priority (soft arbitration).
	ctl := cam.GetOrCreateControl()
	ctlCfg := ctl.GetOrCreateConfig()
	ctlCfg.ControlMode = yangpkg.OpenitsCctvTypes_CctvControlMode_cctv_control_central
	reqHolder := ctlCfg.GetOrCreateHolder()
	reqHolder.RequestedBy = strPtr("op-tmc-07")
	reqHolder.Priority = u8Ptr(200)
	reqHolder.LockoutTimeoutS = u16Ptr(120)
	ctlSt := ctl.GetOrCreateState()
	ctlSt.ControlMode = yangpkg.OpenitsCctvTypes_CctvControlMode_cctv_control_central
	curHolder := ctlSt.GetOrCreateHolder()
	curHolder.CurrentHolder = strPtr("op-tmc-07")
	curHolder.HeldPriority = u8Ptr(200)
	curHolder.HeldSince = strPtr("2026-07-17T14:00:00Z")
	curHolder.ExpiresAt = strPtr("2026-07-17T14:02:00Z")

	return dev, nil
}

func subscribeCctv(ctx context.Context, out chan<- tests.EventEnvelope, window time.Duration) error {
	base := "openits.us-tx.txdot.d07.cctv.cctv-i35-mm214"
	src := "urn:openits:cctv:us-tx:txdot:d07:cctv-i35-mm214"
	events := []tests.EventEnvelope{
		{
			Subject:  base + ".ptz-preset-recalled",
			CEType:   "openits.cctv.ptz-preset-recalled.v1",
			CESource: src,
			CEID:     "01HXYR3K9T8M2NAEQF5P4RCTV1",
			CETime:   time.Now().UTC(),
			Data: &cctvv1.PtzPresetRecalled{
				Kind:       "openits-cctv-types:cctv-ptz-preset-recalled",
				PresetId:   1,
				PresetName: "NB approach",
				RecalledBy: "op-tmc-07",
				ViaTour:    false,
			},
		},
		{
			Subject:  base + ".tour-state-changed",
			CEType:   "openits.cctv.tour-state-changed.v1",
			CESource: src,
			CEID:     "01HXYR3K9T8M2NAEQF5P4RCTV2",
			CETime:   time.Now().UTC(),
			Data: &cctvv1.TourStateChanged{
				Kind:         "openits-cctv-types:cctv-tour-state-changed",
				TourId:       1,
				PreviousState: cctvv1.TourRunState_TOUR_RUN_STATE_STOPPED,
				CurrentState:  cctvv1.TourRunState_TOUR_RUN_STATE_RUNNING,
			},
		},
		{
			Subject:  base + ".lockout-denied",
			CEType:   "openits.cctv.lockout-denied.v1",
			CESource: src,
			CEID:     "01HXYR3K9T8M2NAEQF5P4RCTV3",
			CETime:   time.Now().UTC(),
			Data: &cctvv1.LockoutDenied{
				Kind:              "openits-cctv-types:cctv-lockout-denied",
				RequestedBy:       "op-tmc-12",
				RequestedPriority: 50,
				CurrentHolder:     "op-tmc-07",
				HeldPriority:      200,
			},
		},
		{
			Subject:  base + ".fault-raised",
			CEType:   "openits.cctv.fault-raised.v1",
			CESource: src,
			CEID:     "01HXYR3K9T8M2NAEQF5P4RCTV4",
			CETime:   time.Now().UTC(),
			Data: &commonv1.FaultRaised{
				FaultId: "f-focus-01",
				Kind:    "openits-cctv-types:cctv-fault-focus",
			},
		},
	}
	for _, e := range events {
		select {
		case <-ctx.Done():
			return nil
		case out <- e:
		}
	}
	select {
	case <-ctx.Done():
	case <-time.After(window):
	}
	return nil
}
