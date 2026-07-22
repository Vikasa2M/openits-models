package tests

import (
	"strings"

	yangpkg "github.com/Vikasa2M/openits-models/pkg/yang/openits"
)

// ----- identity / system -----

func TestRSUIdentity_RsuID(t *T, obs *Observation) {
	r := obs.Device.GetRsu()
	if r == nil {
		t.Fatalf("no rsu container present")
	}
	if r.GetConfig().GetId() == "" {
		t.Errorf("rsu/config/id is unset")
	}
}

func TestRSUSystem_Firmware(t *T, obs *Observation) {
	r := obs.Device.GetRsu()
	if r == nil {
		return
	}
	st := r.GetState()
	if st == nil || st.GetFirmware() == "" {
		t.Errorf("state/firmware is unset; required for field-service diagnostics")
	}
}

func TestRSUSystem_HardwareVersion(t *T, obs *Observation) {
	r := obs.Device.GetRsu()
	if r == nil {
		return
	}
	st := r.GetState()
	if st == nil || st.GetHardwareVersion() == "" {
		t.Errorf("state/hardware-version is unset")
	}
}

func TestRSUSystem_SerialNumber(t *T, obs *Observation) {
	r := obs.Device.GetRsu()
	if r == nil {
		return
	}
	st := r.GetState()
	if st == nil || st.GetSerial() == "" {
		t.Errorf("state/serial is unset")
	}
}

// ----- channels -----

func TestRSUChannels_AtLeastOneChannel(t *T, obs *Observation) {
	r := obs.Device.GetRsu()
	if r == nil {
		return
	}
	ch := r.GetChannels()
	if ch == nil || len(ch.Channel) == 0 {
		t.Errorf("no channels configured")
	}
}

func TestRSUChannels_AtLeastOneEnabled(t *T, obs *Observation) {
	r := obs.Device.GetRsu()
	if r == nil {
		return
	}
	ch := r.GetChannels()
	if ch == nil {
		return
	}
	for _, c := range ch.Channel {
		if cfg := c.GetConfig(); cfg != nil && cfg.GetEnabled() {
			return
		}
	}
	t.Errorf("no enabled channel; RSU is effectively offline")
}

// A channel carrying a DSRC channel number must be a DSRC (or dual-mode)
// radio — a DSRC channel on a C-V2X radio is the silently-wrong dual-mode
// config the radio-tech<->channel tie exists to prevent (the schema `when`
// enforces it, but ygot Validate does not evaluate `when`).
func TestRSUChannels_DsrcChannelImpliesDsrcTech(t *T, obs *Observation) {
	r := obs.Device.GetRsu()
	if r == nil || r.GetChannels() == nil {
		return
	}
	dsrc := yangpkg.OpenitsV2XRadio_RadioTech_radio_dsrc
	dual := yangpkg.OpenitsV2XRadio_RadioTech_radio_dual_mode
	for n, c := range r.GetChannels().Channel {
		cfg := c.GetConfig()
		if cfg == nil || cfg.DsrcChannelNumber == nil {
			continue
		}
		if c.RadioTech != dsrc && c.RadioTech != dual {
			t.Errorf("channel %q has DSRC channel number %d but radio-tech is %v (must be DSRC or dual-mode)",
				n, *cfg.DsrcChannelNumber, c.RadioTech)
		}
	}
}

// A decided SRM request must record who decided it. A status of approved /
// active / completed / denied means a grant authority acted, so
// decision-authority must be a real authority (not unset, not 'none') —
// otherwise the grant/denial is unauditable and the NTCIP 1211 TSP-grant-rate
// KPI is unattributable. A grant with no grantor is the defect this guards.
func TestRSUSrm_DecidedRequestHasAuthority(t *T, obs *Observation) {
	r := obs.Device.GetRsu()
	if r == nil || r.GetMessages() == nil || r.GetMessages().GetSrmSsm() == nil {
		return
	}
	ar := r.GetMessages().GetSrmSsm().GetActiveRequests()
	if ar == nil {
		return
	}
	approved := yangpkg.OpenitsV2XMessagingTypes_SrmRequestStatus_approved
	active := yangpkg.OpenitsV2XMessagingTypes_SrmRequestStatus_active
	completed := yangpkg.OpenitsV2XMessagingTypes_SrmRequestStatus_completed
	denied := yangpkg.OpenitsV2XMessagingTypes_SrmRequestStatus_denied
	unset := yangpkg.OpenitsV2XMessagingTypes_DecisionAuthority_UNSET
	none := yangpkg.OpenitsV2XMessagingTypes_DecisionAuthority_none
	for id, req := range ar.Request {
		s := req.Status
		if s != approved && s != active && s != completed && s != denied {
			continue
		}
		if req.DecisionAuthority == unset || req.DecisionAuthority == none {
			t.Errorf("SRM request %q has status %v but no decision-authority — the grant/denial is unauditable", id, s)
		}
	}
}

// A reported GNSS position-deviation is the distance from the surveyed
// reference position; without that reference the number is uninterpretable.
// So if diagnostics reports position-deviation-m, a surveyed-position must be
// configured — otherwise the spoofing/drift signal has nothing to compare
// against. Guards the config<->state tie.
func TestRSUGnss_DeviationRequiresSurveyedPosition(t *T, obs *Observation) {
	r := obs.Device.GetRsu()
	if r == nil || r.GetDiagnostics() == nil {
		return
	}
	if r.GetDiagnostics().PositionDeviationM == nil {
		return
	}
	if r.GetGnss() == nil || r.GetGnss().GetSurveyedPosition() == nil {
		t.Errorf("diagnostics reports position-deviation-m but no gnss/surveyed-position is configured to compute it against")
	}
}

// An operational channel must advertise at least one V2X message type — a
// channel reporting itself operational while carrying nothing on the air is a
// silent gap a consumer treating it as active would miss. Also exercises the
// v2x-message-type vocabulary (including RTCM SC-104 corrections, SAE J2735
// rtcmCorrections) on the ygot-validated config tree.
func TestRSUChannels_OperationalChannelAdvertisesMessageTypes(t *T, obs *Observation) {
	r := obs.Device.GetRsu()
	if r == nil || r.GetChannels() == nil {
		return
	}
	for n, c := range r.GetChannels().Channel {
		st := c.GetState()
		if st == nil || st.Operational == nil || !*st.Operational {
			continue
		}
		if len(c.GetConfig().GetMessageTypes()) == 0 {
			t.Errorf("channel %q is operational but advertises no V2X message types", n)
		}
	}
}

// An application certificate must list at least one PSID permission — a cert
// that can sign for no service is useless, and without the permission list
// "valid cert, wrong PSID" (a message signed by a genuine cert lacking the
// PSID's permission, silently discarded by receivers) is undetectable.
func TestRSUCerts_AppCertHasPermissions(t *T, obs *Observation) {
	r := obs.Device.GetRsu()
	if r == nil || r.GetSecurity() == nil || r.GetSecurity().GetCertificates() == nil {
		return
	}
	app := yangpkg.OpenitsRsu_Rsu_Security_Certificates_Certificate_State_Type_application
	for id, c := range r.GetSecurity().GetCertificates().Certificate {
		st := c.GetState()
		if st == nil || st.Type != app {
			continue
		}
		if len(st.Permissions) == 0 {
			t.Errorf("application certificate %q lists no PSID permissions; it can sign no service and 'valid cert, wrong PSID' is undetectable", id)
		}
	}
}

// Misbehavior-report counts must reconcile: every generated report is either
// sent or still pending, so sent + pending cannot exceed generated.
func TestRSUSecurity_MbrCountsConsistent(t *T, obs *Observation) {
	r := obs.Device.GetRsu()
	if r == nil || r.GetSecurity() == nil {
		return
	}
	mbr := r.GetSecurity().GetState().GetMisbehaviorReporting()
	if mbr == nil || mbr.ReportsGenerated == nil {
		return
	}
	accounted := mbr.GetReportsSent() + uint64(mbr.GetReportsPending())
	if accounted > mbr.GetReportsGenerated() {
		t.Errorf("MBR sent(%d)+pending(%d)=%d exceeds generated(%d)",
			mbr.GetReportsSent(), mbr.GetReportsPending(), accounted, mbr.GetReportsGenerated())
	}
}

// Store-and-Repeat must stop at expiry: a TIM that is still broadcasting while
// its window has closed is a stale-advisory replay (the post-outage defect this
// model exists to make expressible), so broadcasting AND expired must not both
// be true.
func TestRSUTim_BroadcastingImpliesNotExpired(t *T, obs *Observation) {
	r := obs.Device.GetRsu()
	if r == nil || r.GetMessages() == nil || r.GetMessages().GetTim() == nil ||
		r.GetMessages().GetTim().GetActive() == nil {
		return
	}
	for id, m := range r.GetMessages().GetTim().GetActive().Message {
		st := m.GetState()
		if st == nil || st.Broadcasting == nil || st.Expired == nil {
			continue
		}
		if *st.Broadcasting && *st.Expired {
			t.Errorf("TIM %q is broadcasting while expired — a stale advisory is being replayed", id)
		}
	}
}

func TestRSUChannels_ChannelStateReported(t *T, obs *Observation) {
	r := obs.Device.GetRsu()
	if r == nil {
		return
	}
	ch := r.GetChannels()
	if ch == nil {
		return
	}
	for n, c := range ch.Channel {
		cfg := c.GetConfig()
		if cfg == nil || !cfg.GetEnabled() {
			continue
		}
		st := c.GetState()
		if st == nil {
			t.Errorf("channel %s enabled but has no state container", n)
			continue
		}
		if st.Operational == nil {
			t.Errorf("channel %s missing state/operational", n)
		}
	}
}

// ----- diagnostics -----

func TestRSUDiagnostics_GPSFix(t *T, obs *Observation) {
	r := obs.Device.GetRsu()
	if r == nil {
		return
	}
	d := r.GetDiagnostics()
	if d == nil {
		return
	}
	switch d.GpsStatus {
	case yangpkg.OpenitsRsuTypes_GpsFixStatus_UNSET,
		yangpkg.OpenitsRsuTypes_GpsFixStatus_no_fix:
		t.Errorf("GPS status %v — RSU timing will be unreliable", d.GpsStatus)
	}
}

func TestRSUDiagnostics_SatellitesVisible(t *T, obs *Observation) {
	r := obs.Device.GetRsu()
	if r == nil {
		return
	}
	d := r.GetDiagnostics()
	if d == nil || d.SatellitesVisible == nil {
		return
	}
	// A claimed fix with fewer than 4 satellites is definitionally a
	// dead reckoning / last-known-good reading, not a real fix.
	switch d.GpsStatus {
	case yangpkg.OpenitsRsuTypes_GpsFixStatus_fix_2d,
		yangpkg.OpenitsRsuTypes_GpsFixStatus_fix_3d:
		if *d.SatellitesVisible < 4 {
			t.Errorf("GPS reports %v but only %d satellites visible (< 4)",
				d.GpsStatus, *d.SatellitesVisible)
		}
	}
}

func TestRSUDiagnostics_TimeSource(t *T, obs *Observation) {
	r := obs.Device.GetRsu()
	if r == nil {
		return
	}
	d := r.GetDiagnostics()
	if d == nil {
		return
	}
	if d.TimeSource == 0 {
		t.Errorf("diagnostics/time-source unset; SPaT and BSM timestamps cannot be trusted")
	}
}

// ----- SRM/SSM decisions -----

func TestRSUDecisions_SrmDecisionRoundTrips(t *T, obs *Observation) {
	r := obs.Device.GetRsu()
	if r == nil {
		return
	}
	msgs := r.GetMessages()
	if msgs == nil {
		t.Errorf("rsu/messages container missing")
		return
	}
	d := msgs.GetSrmSsm().GetDecisions().GetDecision("srm-000482")
	if d == nil {
		t.Errorf("srm-ssm/decisions/decision[request-id=srm-000482] not found")
		return
	}
	if want := yangpkg.OpenitsRsu_Rsu_Messages_SrmSsm_Decisions_Decision_Action_approve; d.Action != want {
		t.Errorf("decision srm-000482 action = %v, want %v", d.Action, want)
	}
	if got, want := d.GetReason(), "transit priority"; got != want {
		t.Errorf("decision srm-000482 reason = %q, want %q", got, want)
	}
}

// ----- vehicle-analytics -----
//
// Onboard vehicle-analytics (config false, behind if-feature
// onboard-detection, enabled by default) is the RSU's self-reported
// BSM-derived vehicle counts/speeds. sample-basis/sample-count > 0
// distinguishes a real observation window from an unpopulated stub;
// penetration-estimate-pct must fall within [0,100] since it is a
// percentage; counts/count-basis must be set so consumers know
// whether the counts are deduplicated or raw per-session.

func TestRSUAnalytics_SampleBasisPresent(t *T, obs *Observation) {
	r := obs.Device.GetRsu()
	if r == nil {
		return
	}
	basis := r.GetDiagnostics().GetVehicleAnalytics().GetSampleBasis()
	if basis == nil || basis.SampleCount == nil || basis.GetSampleCount() == 0 {
		t.Errorf("diagnostics/vehicle-analytics/sample-basis/sample-count is unset or zero")
	}
}

func TestRSUAnalytics_PenetrationInRange(t *T, obs *Observation) {
	r := obs.Device.GetRsu()
	if r == nil {
		return
	}
	basis := r.GetDiagnostics().GetVehicleAnalytics().GetSampleBasis()
	if basis == nil || basis.PenetrationEstimatePct == nil {
		return
	}
	pct := basis.GetPenetrationEstimatePct()
	if pct < 0 || pct > 100 {
		t.Errorf("sample-basis/penetration-estimate-pct %v out of range [0,100]", pct)
	}
}

func TestRSUAnalytics_CountBasisPresent(t *T, obs *Observation) {
	r := obs.Device.GetRsu()
	if r == nil {
		return
	}
	counts := r.GetDiagnostics().GetVehicleAnalytics().GetCounts()
	if counts == nil || counts.CountBasis == yangpkg.OpenitsRsu_Rsu_Diagnostics_VehicleAnalytics_Counts_CountBasis_UNSET {
		t.Errorf("diagnostics/vehicle-analytics/counts/count-basis is unset")
	}
}

// ----- event shapes -----

func TestRSUEvent_SrmReceivedShape(t *T, obs *Observation) {
	for _, e := range obs.Events {
		if !strings.HasSuffix(e.Subject, ".rsu-srm-received") {
			continue
		}
		want := "openits.rsu.rsu-srm-received.v1"
		if e.CEType != want {
			t.Errorf("rsu-srm-received ce-type %q, want %q", e.CEType, want)
		}
		return
	}
	t.Errorf("no rsu-srm-received event observed during %s window", obs.Window)
}

func TestRSUEvent_CertificateExpiringShape(t *T, obs *Observation) {
	for _, e := range obs.Events {
		if !strings.HasSuffix(e.Subject, ".rsu-certificate-expiring") {
			continue
		}
		want := "openits.rsu.rsu-certificate-expiring.v1"
		if e.CEType != want {
			t.Errorf("rsu-certificate-expiring ce-type %q, want %q", e.CEType, want)
		}
		return
	}
}

func TestRSUEvent_ChannelFaultShape(t *T, obs *Observation) {
	for _, e := range obs.Events {
		if !strings.HasSuffix(e.Subject, ".rsu-channel-fault") {
			continue
		}
		want := "openits.rsu.rsu-channel-fault.v1"
		if e.CEType != want {
			t.Errorf("rsu-channel-fault ce-type %q, want %q", e.CEType, want)
		}
		return
	}
}

func TestRSUEvent_GpsStatusChangeShape(t *T, obs *Observation) {
	for _, e := range obs.Events {
		if !strings.HasSuffix(e.Subject, ".rsu-gps-status-change") {
			continue
		}
		want := "openits.rsu.rsu-gps-status-change.v1"
		if e.CEType != want {
			t.Errorf("rsu-gps-status-change ce-type %q, want %q", e.CEType, want)
		}
		return
	}
}

func TestRSUEvent_SecurityEventShape(t *T, obs *Observation) {
	for _, e := range obs.Events {
		if !strings.HasSuffix(e.Subject, ".rsu-security-event") {
			continue
		}
		want := "openits.rsu.rsu-security-event.v1"
		if e.CEType != want {
			t.Errorf("rsu-security-event ce-type %q, want %q", e.CEType, want)
		}
		return
	}
}

// An expedited EVP auto-grant is only trustworthy if the RSU is signing its
// messages: a request decided by the evp-auto authority implies the RSU has
// security-enabled true. An unsigned auto-grant is the defect this guards.
func TestRSUSrm_EvpGrantRequiresSigning(t *T, obs *Observation) {
	r := obs.Device.GetRsu()
	if r == nil || r.GetMessages() == nil || r.GetMessages().GetSrmSsm() == nil {
		return
	}
	ar := r.GetMessages().GetSrmSsm().GetActiveRequests()
	if ar == nil {
		return
	}
	hasEvpAuto := false
	for _, req := range ar.Request {
		if req.DecisionAuthority == yangpkg.OpenitsV2XMessagingTypes_DecisionAuthority_evp_auto {
			hasEvpAuto = true
			break
		}
	}
	if !hasEvpAuto {
		return
	}
	sec := r.GetSecurity()
	if sec == nil || sec.GetConfig() == nil || !sec.GetConfig().GetSecurityEnabled() {
		t.Errorf("an SRM request is decided by evp-auto authority but security-enabled is not true")
	}
}

// Every enabled SPaT intersection must have a matching MAP intersection: a
// SPaT phase-state broadcast is meaningless to a vehicle without the MAP
// geometry for the same (region, id).
func TestRSUMessages_SpatIntersectionsSubsetOfMap(t *T, obs *Observation) {
	msgs := obs.Device.GetRsu().GetMessages()
	if msgs == nil || msgs.GetSpat() == nil || msgs.GetSpat().GetConfig() == nil {
		return
	}
	mapSet := map[[2]uint16]bool{}
	if msgs.GetMap() != nil && msgs.GetMap().GetConfig() != nil {
		for k := range msgs.GetMap().GetConfig().Intersection {
			mapSet[[2]uint16{k.Region, k.Id}] = true
		}
	}
	for k := range msgs.GetSpat().GetConfig().Intersection {
		if !mapSet[[2]uint16{k.Region, k.Id}] {
			t.Errorf("SPaT intersection region=%d id=%d has no matching MAP intersection", k.Region, k.Id)
		}
	}
}
