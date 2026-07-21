// Package tests contains the OpenITS conformance test functions.  Each
// Test* function takes a shared *Observation and records PASS/FAIL on
// it.  The runner in the parent package drives execution and prints a
// report.
package tests

import (
	"fmt"
	"time"

	yangpkg "github.com/Vikasa2M/openits-models/pkg/yang/openits"
	"google.golang.org/protobuf/proto"
)

// EventEnvelope is a CloudEvents-in-NATS observation the driver
// captures during the subscription window.
type EventEnvelope struct {
	Subject  string
	CEType   string
	CESource string
	CEID     string
	CETime   time.Time
	Data     proto.Message
}

// Observation is the shared state every test function reads.
type Observation struct {
	Device *yangpkg.Device
	Events []EventEnvelope
	Window time.Duration
}

// Result records the outcome of one check.
type Result struct {
	Name    string
	Pass    bool
	Message string
}

// T is the conformance-test shim (inspired by testing.T but purpose-
// built: we don't want test registration via `go test`, we want a
// standalone binary).
type T struct {
	name  string
	fails []string
}

func (t *T) Errorf(format string, a ...any) { t.fails = append(t.fails, fmt.Sprintf(format, a...)) }
func (t *T) Fatalf(format string, a ...any) { t.Errorf(format, a...); panic(fatal{}) }

type fatal struct{}

// Run executes fn and captures the result.
func Run(name string, fn func(*T, *Observation), obs *Observation) (r Result) {
	t := &T{name: name}
	defer func() {
		if x := recover(); x != nil {
			if _, ok := x.(fatal); !ok {
				panic(x)
			}
		}
		r.Name = name
		if len(t.fails) == 0 {
			r.Pass = true
		} else {
			r.Message = fmtFails(t.fails)
		}
	}()
	fn(t, obs)
	return
}

func fmtFails(f []string) string {
	if len(f) == 1 {
		return f[0]
	}
	out := ""
	for i, s := range f {
		if i > 0 {
			out += "; "
		}
		out += s
	}
	return out
}

// TestCase binds a name to a check function.  Adding a new check is
// one line in All() plus defining the function.
type TestCase struct {
	Name string
	Fn   func(*T, *Observation)
}

// All returns the canonical list of checks for the given device kind.
// Envelope-shape checks are universal; kind-specific checks live behind
// the switch.
func All(kind string) []TestCase {
	common := []TestCase{
		// envelope shape — applies to every service
		{"TestSubject_SevenTokenShape", TestSubject_SevenTokenShape},
		{"TestSubject_OpenitsPrefix", TestSubject_OpenitsPrefix},
		{"TestCEType_OpenitsFormat", TestCEType_OpenitsFormat},
		{"TestCESource_URN", TestCESource_URN},
		{"TestCEID_Present", TestCEID_Present},

		// YANG validation — ygot struct must parse whatever the driver
		// collected for the device under test
		{"TestYANG_Validate", TestYANG_Validate},
	}

	switch kind {
	case "ramp-metering":
		return append(common,
			// identity
			TestCase{"TestRMIdentity_MeterID", TestRMIdentity_MeterID},
			TestCase{"TestRMIdentity_Firmware", TestRMIdentity_Firmware},
			TestCase{"TestRMIdentity_MakeModel", TestRMIdentity_MakeModel},

			// operational status
			TestCase{"TestRMOperational_ModePresent", TestRMOperational_ModePresent},
			TestCase{"TestRMOperational_ReleaseRatePositiveWhenActive", TestRMOperational_ReleaseRatePositiveWhenActive},
			TestCase{"TestRMOperational_ActivePlanExists", TestRMOperational_ActivePlanExists},

			// plans
			TestCase{"TestRMPlans_AtLeastOne", TestRMPlans_AtLeastOne},
			TestCase{"TestRMPlans_ActivePlanTimingSane", TestRMPlans_ActivePlanTimingSane},
			TestCase{"TestRMPlans_HeadwayFitsTiming", TestRMPlans_HeadwayFitsTiming},
			TestCase{"TestRMPlans_HeadwayConsistentWithRate", TestRMPlans_HeadwayConsistentWithRate},
			TestCase{"TestRMPlans_QueueOverrideHysteresis", TestRMPlans_QueueOverrideHysteresis},
			TestCase{"TestRMControl_CommandSourcePresent", TestRMControl_CommandSourcePresent},

			// lanes / detectors
			TestCase{"TestRMLanes_AtLeastOneMeteredLane", TestRMLanes_AtLeastOneMeteredLane},
			TestCase{"TestRMLanes_DemandAndPassageDetectors", TestRMLanes_DemandAndPassageDetectors},

			// events
			TestCase{"TestRMEvent_ModeChangedShape", TestRMEvent_ModeChangedShape},
			TestCase{"TestRMEvent_ReleaseRateChangedShape", TestRMEvent_ReleaseRateChangedShape},
			TestCase{"TestRMEvent_QueueOverrideActivatedShape", TestRMEvent_QueueOverrideActivatedShape},
		)
	case "rsu":
		return append(common,
			// identity / system
			TestCase{"TestRSUIdentity_RsuID", TestRSUIdentity_RsuID},
			TestCase{"TestRSUSystem_Firmware", TestRSUSystem_Firmware},
			TestCase{"TestRSUSystem_HardwareVersion", TestRSUSystem_HardwareVersion},
			TestCase{"TestRSUSystem_SerialNumber", TestRSUSystem_SerialNumber},

			// channels
			TestCase{"TestRSUChannels_AtLeastOneChannel", TestRSUChannels_AtLeastOneChannel},
			TestCase{"TestRSUChannels_AtLeastOneEnabled", TestRSUChannels_AtLeastOneEnabled},
			TestCase{"TestRSUChannels_ChannelStateReported", TestRSUChannels_ChannelStateReported},
			TestCase{"TestRSUChannels_DsrcChannelImpliesDsrcTech", TestRSUChannels_DsrcChannelImpliesDsrcTech},
			TestCase{"TestRSUChannels_OperationalChannelAdvertisesMessageTypes", TestRSUChannels_OperationalChannelAdvertisesMessageTypes},
			TestCase{"TestRSUGnss_DeviationRequiresSurveyedPosition", TestRSUGnss_DeviationRequiresSurveyedPosition},
			TestCase{"TestRSUSrm_DecidedRequestHasAuthority", TestRSUSrm_DecidedRequestHasAuthority},
			TestCase{"TestRSUSrm_EvpGrantRequiresSigning", TestRSUSrm_EvpGrantRequiresSigning},
			TestCase{"TestRSUMessages_SpatIntersectionsSubsetOfMap", TestRSUMessages_SpatIntersectionsSubsetOfMap},
			TestCase{"TestRSUCerts_AppCertHasPermissions", TestRSUCerts_AppCertHasPermissions},
			TestCase{"TestRSUSecurity_MbrCountsConsistent", TestRSUSecurity_MbrCountsConsistent},
			TestCase{"TestRSUTim_BroadcastingImpliesNotExpired", TestRSUTim_BroadcastingImpliesNotExpired},

			// diagnostics
			TestCase{"TestRSUDiagnostics_GPSFix", TestRSUDiagnostics_GPSFix},
			TestCase{"TestRSUDiagnostics_SatellitesVisible", TestRSUDiagnostics_SatellitesVisible},
			TestCase{"TestRSUDiagnostics_TimeSource", TestRSUDiagnostics_TimeSource},

			// SRM/SSM decisions
			TestCase{"TestRSUDecisions_SrmDecisionRoundTrips", TestRSUDecisions_SrmDecisionRoundTrips},

			// vehicle-analytics
			TestCase{"TestRSUAnalytics_SampleBasisPresent", TestRSUAnalytics_SampleBasisPresent},
			TestCase{"TestRSUAnalytics_PenetrationInRange", TestRSUAnalytics_PenetrationInRange},
			TestCase{"TestRSUAnalytics_CountBasisPresent", TestRSUAnalytics_CountBasisPresent},

			// events
			TestCase{"TestRSUEvent_SrmReceivedShape", TestRSUEvent_SrmReceivedShape},
			TestCase{"TestRSUEvent_CertificateExpiringShape", TestRSUEvent_CertificateExpiringShape},
			TestCase{"TestRSUEvent_ChannelFaultShape", TestRSUEvent_ChannelFaultShape},
			TestCase{"TestRSUEvent_GpsStatusChangeShape", TestRSUEvent_GpsStatusChangeShape},
			TestCase{"TestRSUEvent_SecurityEventShape", TestRSUEvent_SecurityEventShape},
		)
	case "ess":
		return append(common,
			// identity
			TestCase{"TestESSIdentity_StationID", TestESSIdentity_StationID},
			TestCase{"TestESSIdentity_Firmware", TestESSIdentity_Firmware},
			TestCase{"TestESSIdentity_MakeModel", TestESSIdentity_MakeModel},
			TestCase{"TestESSIdentity_Location", TestESSIdentity_Location},

			// atmospheric
			TestCase{"TestESSAtmospheric_HumidityBound", TestESSAtmospheric_HumidityBound},
			TestCase{"TestESSAtmospheric_WindDirectionBound", TestESSAtmospheric_WindDirectionBound},
			TestCase{"TestESSAtmospheric_TemperatureSanity", TestESSAtmospheric_TemperatureSanity},
			TestCase{"TestESSAtmospheric_GustNotLessThanSpeed", TestESSAtmospheric_GustNotLessThanSpeed},
			TestCase{"TestESSAtmospheric_GustNotLessThanAverage", TestESSAtmospheric_GustNotLessThanAverage},

			// precipitation
			TestCase{"TestESSPrecipitation_TypeIntensityConsistency", TestESSPrecipitation_TypeIntensityConsistency},

			// pavement
			TestCase{"TestESSPavement_AtLeastOneSensor", TestESSPavement_AtLeastOneSensor},
			TestCase{"TestESSPavement_WaterDepthNonNegative", TestESSPavement_WaterDepthNonNegative},
			TestCase{"TestESSPavement_ChemicalDepressesFreezePoint", TestESSPavement_ChemicalDepressesFreezePoint},

			// diagnostics
			TestCase{"TestESSDiagnostics_SensorsPresent", TestESSDiagnostics_SensorsPresent},
			TestCase{"TestESSDiagnostics_ObservationFreshness", TestESSDiagnostics_ObservationFreshness},

			// events
			TestCase{"TestESSEvent_FaultRaisedShape", TestESSEvent_FaultRaisedShape},
			TestCase{"TestESSEvent_WeatherAlertShape", TestESSEvent_WeatherAlertShape},
			TestCase{"TestESSEvent_SensorRecalibratedShape", TestESSEvent_SensorRecalibratedShape},
		)
	case "dms":
		return append(common,
			// identity
			TestCase{"TestDMSIdentity_SignID", TestDMSIdentity_SignID},
			TestCase{"TestDMSIdentity_Firmware", TestDMSIdentity_Firmware},
			TestCase{"TestDMSIdentity_MakeModel", TestDMSIdentity_MakeModel},
			TestCase{"TestDMSIdentity_Location", TestDMSIdentity_Location},
			TestCase{"TestDMSIdentity_Dimensions", TestDMSIdentity_Dimensions},
			TestCase{"TestDMSCapabilities_SignTypePresent", TestDMSCapabilities_SignTypePresent},
			TestCase{"TestDMSCapabilities_CharMatrixHasCellSize", TestDMSCapabilities_CharMatrixHasCellSize},
			TestCase{"TestDMSControl_IlluminationControlPresent", TestDMSControl_IlluminationControlPresent},
			TestCase{"TestDMSDiagnostics_StuckPixelsWithinFailed", TestDMSDiagnostics_StuckPixelsWithinFailed},
			TestCase{"TestDMSSchedule_DayPlanHasAction", TestDMSSchedule_DayPlanHasAction},

			// operational status
			TestCase{"TestDMSOperational_ModePresent", TestDMSOperational_ModePresent},
			TestCase{"TestDMSOperational_BrightnessInRange", TestDMSOperational_BrightnessInRange},
			TestCase{"TestDMSOperational_DisplayStatePresent", TestDMSOperational_DisplayStatePresent},
			TestCase{"TestDMSOperational_Heartbeat", TestDMSOperational_Heartbeat},

			// messages
			TestCase{"TestDMSMessages_BufferIntegrity", TestDMSMessages_BufferIntegrity},
			TestCase{"TestDMSMessages_ActiveMatchesSlot", TestDMSMessages_ActiveMatchesSlot},
			TestCase{"TestDMSMessages_SlotStatusValid", TestDMSMessages_SlotStatusValid},
			TestCase{"TestDMSMessages_ActiveBeaconMatchesSlot", TestDMSMessages_ActiveBeaconMatchesSlot},

			// diagnostics
			TestCase{"TestDMSDiagnostics_PixelsFailedBound", TestDMSDiagnostics_PixelsFailedBound},
			TestCase{"TestDMSDiagnostics_LampsFailedBound", TestDMSDiagnostics_LampsFailedBound},

			// fallback
			TestCase{"TestDMSFallback_PowerLossActivePresent", TestDMSFallback_PowerLossActivePresent},

			// events
			TestCase{"TestDMSEvent_FaultRaisedShape", TestDMSEvent_FaultRaisedShape},
			TestCase{"TestDMSEvent_ModeChangedShape", TestDMSEvent_ModeChangedShape},
			TestCase{"TestDMSEvent_ActivationFailedShape", TestDMSEvent_ActivationFailedShape},
		)
	case "traffic-sensor":
		return append(common,
			// identity
			TestCase{"TestTrafficSensorIdentity_SensorID", TestTrafficSensorIdentity_SensorID},

			// configuration
			TestCase{"TestTrafficSensorConfig_LaneNumberingOrigin", TestTrafficSensorConfig_LaneNumberingOrigin},

			// lanes
			TestCase{"TestTrafficSensorLane_ZoneLengthPositive", TestTrafficSensorLane_ZoneLengthPositive},

			// interval data
			TestCase{"TestTrafficSensorInterval_DataQualityPresent", TestTrafficSensorInterval_DataQualityPresent},
			TestCase{"TestTrafficSensorInterval_UptimeInRange", TestTrafficSensorInterval_UptimeInRange},
			TestCase{"TestTrafficSensorInterval_SpaceMeanNotAboveTimeMean", TestTrafficSensorInterval_SpaceMeanNotAboveTimeMean},

			// classification integrity
			TestCase{"TestTrafficSensorConfig_ClassificationSchemePresent", TestTrafficSensorConfig_ClassificationSchemePresent},
			TestCase{"TestTrafficSensorInterval_ClassVolumeReconciles", TestTrafficSensorInterval_ClassVolumeReconciles},

			// operational health + queue
			TestCase{"TestTrafficSensorLane_OperationalStatusPresent", TestTrafficSensorLane_OperationalStatusPresent},
			TestCase{"TestTrafficSensorHealth_RollupConsistent", TestTrafficSensorHealth_RollupConsistent},
			TestCase{"TestTrafficSensorQueue_LengthWhenQueueing", TestTrafficSensorQueue_LengthWhenQueueing},

			// calibration + mounting detail
			TestCase{"TestTrafficSensorDiag_CalibrationStatusKnown", TestTrafficSensorDiag_CalibrationStatusKnown},
			TestCase{"TestTrafficSensorConfig_MountingSideForFromSensor", TestTrafficSensorConfig_MountingSideForFromSensor},

			// events
			TestCase{"TestTrafficSensorEvent_IntervalReportShape", TestTrafficSensorEvent_IntervalReportShape},
			TestCase{"TestTrafficSensorEvent_FaultRaisedShape", TestTrafficSensorEvent_FaultRaisedShape},
		)
	case "reversible-lane":
		return append(common,
			// identity
			TestCase{"TestReversibleLaneIdentity_FacilityID", TestReversibleLaneIdentity_FacilityID},

			// interlocks
			TestCase{"TestReversibleLaneInterlock_HasKind", TestReversibleLaneInterlock_HasKind},

			// control / changeover clearance gate
			TestCase{"TestReversibleLaneControl_ChangeoverPermittedPresent", TestReversibleLaneControl_ChangeoverPermittedPresent},
			TestCase{"TestReversibleLaneControl_BlockingConsistent", TestReversibleLaneControl_BlockingConsistent},

			// segments / lanes
			TestCase{"TestReversibleLaneLane_GreenImpliesOpposingRedX", TestReversibleLaneLane_GreenImpliesOpposingRedX},
			TestCase{"TestReversibleLaneLane_GateStatePresent", TestReversibleLaneLane_GateStatePresent},
			TestCase{"TestRL_DirectionOpenGateBlocksReverse", TestRL_DirectionOpenGateBlocksReverse},

			// events
			TestCase{"TestReversibleLaneEvent_LcsConflictShape", TestReversibleLaneEvent_LcsConflictShape},
			TestCase{"TestReversibleLaneEvent_FaultRaisedShape", TestReversibleLaneEvent_FaultRaisedShape},
		)
	case "perception":
		return append(common,
			// identity
			TestCase{"TestPerceptionIdentity_SensorID", TestPerceptionIdentity_SensorID},

			// incident semantics
			TestCase{"TestPerceptionIncident_SeverityPresent", TestPerceptionIncident_SeverityPresent},
			TestCase{"TestPerceptionIncident_ConfidencePresent", TestPerceptionIncident_ConfidencePresent},

			// track lifecycle
			TestCase{"TestPerceptionTrack_LifecyclePresent", TestPerceptionTrack_LifecyclePresent},

			// incident-review disposition round-trip
			TestCase{"TestPerception_DispositionRoundTrip", TestPerception_DispositionRoundTrip},

			// events
			TestCase{"TestPerceptionEvent_IncidentDetectedShape", TestPerceptionEvent_IncidentDetectedShape},
			TestCase{"TestPerceptionEvent_IntervalCrossedReconciles", TestPerceptionEvent_IntervalCrossedReconciles},
			TestCase{"TestPerceptionEvent_FaultRaisedShape", TestPerceptionEvent_FaultRaisedShape},
		)
	case "cctv":
		return append(common,
			// identity
			TestCase{"TestCctvIdentity_CameraID", TestCctvIdentity_CameraID},

			// PTZ
			TestCase{"TestCctvPtz_ActivePresetIsDefined", TestCctvPtz_ActivePresetIsDefined},
			TestCase{"TestCctvTour_RunningImpliesActiveTour", TestCctvTour_RunningImpliesActiveTour},
			TestCase{"TestCctv_VelocityMoveRequiresVelocityConfig", TestCctv_VelocityMoveRequiresVelocityConfig},
			TestCase{"TestCctv_OperationalStatusPresent", TestCctv_OperationalStatusPresent},

			// streams
			TestCase{"TestCctvStream_OkHealthHasBitrate", TestCctvStream_OkHealthHasBitrate},

			// control ownership
			TestCase{"TestCctvControl_HeldImpliesHolder", TestCctvControl_HeldImpliesHolder},

			// events
			TestCase{"TestCctvEvent_PresetRecalledShape", TestCctvEvent_PresetRecalledShape},
			TestCase{"TestCctvEvent_LockoutDeniedShape", TestCctvEvent_LockoutDeniedShape},
			TestCase{"TestCctvEvent_FaultRaisedShape", TestCctvEvent_FaultRaisedShape},
		)
	default: // "asc"
		return append(common,
			// identity
			TestCase{"TestIdentity_ControllerID", TestIdentity_ControllerID},
			TestCase{"TestIdentity_Firmware", TestIdentity_Firmware},
			TestCase{"TestIdentity_MakeModel", TestIdentity_MakeModel},
			TestCase{"TestIdentity_Location", TestIdentity_Location},

			// phase timing
			TestCase{"TestPhaseTiming_HasPhases", TestPhaseTiming_HasPhases},
			TestCase{"TestPhaseTiming_YellowChangeMinimum", TestPhaseTiming_YellowChangeMinimum},
			TestCase{"TestPhaseTiming_RedClearMinimum", TestPhaseTiming_RedClearMinimum},
			TestCase{"TestPhaseTiming_MinGreenMinimum", TestPhaseTiming_MinGreenMinimum},
			TestCase{"TestPhaseTiming_MaxGreenSane", TestPhaseTiming_MaxGreenSane},
			TestCase{"TestPhaseTiming_PedClearSane", TestPhaseTiming_PedClearSane},

			// detectors
			TestCase{"TestDetectors_AtLeastOne", TestDetectors_AtLeastOne},
			TestCase{"TestDetectors_AssignedToPhase", TestDetectors_AssignedToPhase},
			TestCase{"TestDetectors_MeasurementReported", TestDetectors_MeasurementReported},

			// overlaps (incl. FYA)
			TestCase{"TestOverlaps_AtLeastOne", TestOverlaps_AtLeastOne},
			TestCase{"TestOverlaps_FYAProtectedAndOpposingPhasesResolve", TestOverlaps_FYAProtectedAndOpposingPhasesResolve},

			// channels
			TestCase{"TestChannels_AtLeastOne", TestChannels_AtLeastOne},
			TestCase{"TestChannels_SourceResolves", TestChannels_SourceResolves},

			// conflict monitor
			TestCase{"TestConflictMonitor_AtLeastOnePermissive", TestConflictMonitor_AtLeastOnePermissive},
			TestCase{"TestConflictMonitor_PermissiveResolvesChannels", TestConflictMonitor_PermissiveResolvesChannels},
			TestCase{"TestConflictMonitor_PermissiveCanonicalOrder", TestConflictMonitor_PermissiveCanonicalOrder},
			TestCase{"TestConflictMonitor_NoSameRingPermissive", TestConflictMonitor_NoSameRingPermissive},

			// coordination
			TestCase{"TestCoordination_ActivePlan", TestCoordination_ActivePlan},
			TestCase{"TestCoordination_NEMADualRing", TestCoordination_NEMADualRing},
			TestCase{"TestCoordination_BarrierAssignment", TestCoordination_BarrierAssignment},
			TestCase{"TestCoordination_SplitsWithinCycle", TestCoordination_SplitsWithinCycle},
			TestCase{"TestCoordination_BarrierCrossingAlignment", TestCoordination_BarrierCrossingAlignment},

			// timebase
			TestCase{"TestTimebase_ReferencesResolve", TestTimebase_ReferencesResolve},

			// preemption
			TestCase{"TestPreemption_EventFired", TestPreemption_EventFired},
			TestCase{"TestPreemption_EventTypeShape", TestPreemption_EventTypeShape},
			TestCase{"TestPreemption_RailTrackClearance", TestPreemption_RailTrackClearance},

			// health
			TestCase{"TestHealth_OperationalStatus", TestHealth_OperationalStatus},
			TestCase{"TestHealth_NotFlashing", TestHealth_NotFlashing},

			// cabinet power / UPS (platform capability)
			TestCase{"TestCabinetPower_SourceReported", TestCabinetPower_SourceReported},
			TestCase{"TestCabinetPower_BatteryChargeReported", TestCabinetPower_BatteryChargeReported},
			TestCase{"TestCabinetPower_OnBatteryHasRuntime", TestCabinetPower_OnBatteryHasRuntime},
		)
	}
}
