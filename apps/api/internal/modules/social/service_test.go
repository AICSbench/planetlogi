package social

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AICSbench/planetlogi/apps/api/internal/platform/economy"
)

func TestStarDuelsEnergyLimitAndReset(t *testing.T) {
	ledger := economy.NewMemoryLedger()
	svc := NewService(ledger)
	ctx := context.Background()

	day1 := time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC)
	svc.SetClockForTest(func() time.Time { return day1 })

	for i := 1; i <= DailyEnergyMax; i++ {
		duel, left, err := svc.StartDuel(ctx, "stu-1", 1, "Asia/Jakarta", "en", "RANDOM", "")
		if err != nil {
			t.Fatalf("expected duel %d to start, got err: %v", i, err)
		}
		if left != DailyEnergyMax-i {
			t.Fatalf("expected %d energy left, got %d", DailyEnergyMax-i, left)
		}
		if duel.Opponent.Grade != 1 {
			t.Fatalf("random matchmaking violated grade isolation: got grade %d", duel.Opponent.Grade)
		}
		if strings.Contains(duel.Opponent.MaskedName, "@") || !strings.HasSuffix(duel.Opponent.MaskedName, ".") {
			t.Fatalf("opponent name not masked: %q", duel.Opponent.MaskedName)
		}
	}

	if _, _, err := svc.StartDuel(ctx, "stu-1", 1, "Asia/Jakarta", "en", "RANDOM", ""); err != ErrNoEnergy {
		t.Fatalf("expected ErrNoEnergy on 4th duel, got %v", err)
	}

	day2 := day1.Add(24 * time.Hour)
	svc.SetClockForTest(func() time.Time { return day2 })
	_, left, err := svc.StartDuel(ctx, "stu-1", 1, "Asia/Jakarta", "en", "CLASSMATE", "")
	if err != nil || left != 2 {
		t.Fatalf("expected energy reset on next day, err=%v left=%d", err, left)
	}
}

func TestStarDuelGradingAndIdempotentSP(t *testing.T) {
	ctx := context.Background()
	ledger := economy.NewMemoryLedger(economy.Wallet{StudentID: "stu-2", TotalSP: 10})
	svc := NewService(ledger)

	duel, _, err := svc.StartDuel(ctx, "stu-2", 2, "Asia/Jakarta", "id", "CLASSMATE", "")
	if err != nil {
		t.Fatalf("StartDuel: %v", err)
	}
	res1, err := svc.SubmitDuel(ctx, "stu-2", duel.ID, "id", map[int]map[string]string{})
	if err != nil {
		t.Fatalf("SubmitDuel: %v", err)
	}
	if res1.Status != "COMPLETED" || res1.RewardSP <= 0 {
		t.Fatalf("expected completed duel with participation SP, got %+v", res1)
	}

	// Verify the wallet was actually credited with the participation reward.
	probe1, err := ledger.Apply(ctx, economy.MutationRequest{
		StudentID:      "stu-2",
		Currency:       economy.CurrencySP,
		Delta:          1,
		SourceType:     "PROBE",
		IdempotencyKey: "probe-1",
	})
	if err != nil {
		t.Fatalf("probe ledger: %v", err)
	}
	if want := int64(10) + res1.RewardSP + 1; probe1.Wallet.TotalSP != want {
		t.Fatalf("expected wallet %d (initial 10 + reward %d + probe 1), got %d", want, res1.RewardSP, probe1.Wallet.TotalSP)
	}

	// Replay submit is idempotent
	_, err = svc.SubmitDuel(ctx, "stu-2", duel.ID, "id", map[int]map[string]string{})
	if err != nil {
		t.Fatalf("replay SubmitDuel: %v", err)
	}
	probe2, err := ledger.Apply(ctx, economy.MutationRequest{
		StudentID:      "stu-2",
		Currency:       economy.CurrencySP,
		Delta:          1,
		SourceType:     "PROBE",
		IdempotencyKey: "probe-2",
	})
	if err != nil {
		t.Fatalf("probe ledger 2: %v", err)
	}
	if probe2.Wallet.TotalSP != probe1.Wallet.TotalSP+1 {
		t.Fatalf("expected idempotent SP award (only +1 from probe), got %d then %d", probe1.Wallet.TotalSP, probe2.Wallet.TotalSP)
	}
}

func TestStarCrewCodeUniquenessAndJoin(t *testing.T) {
	svc := NewService(economy.NewMemoryLedger())
	ctx := context.Background()

	seen := make(map[string]bool)
	for i := 0; i < 20; i++ {
		c := svc.GetOrCreateCrew(ctx, "pilot-"+string(rune('a'+i)), "Raka Pratama", 1)
		if len(c.CrewCode) != 9 {
			t.Fatalf("expected 9-digit crew code, got %q", c.CrewCode)
		}
		if seen[c.CrewCode] {
			t.Fatalf("duplicate crew code generated: %s", c.CrewCode)
		}
		seen[c.CrewCode] = true
	}

	joined, err := svc.JoinCrew(ctx, "stu-join", "Siti Rahma", 1, "482-910-375")
	if err != nil {
		t.Fatalf("JoinCrew failed: %v", err)
	}
	if len(joined.Members) != 3 {
		t.Fatalf("expected 3 crew members after join, got %d", len(joined.Members))
	}
	if joined.WeeklyTotalSP != 68+52+35 {
		t.Fatalf("unexpected weekly crew SP aggregation: %d", joined.WeeklyTotalSP)
	}
	left := svc.LeaveCrew(ctx, "stu-join", "Siti Rahma", 1)
	if left.CrewCode == "482910375" || len(left.Members) != 1 {
		t.Fatalf("expected fresh personal crew after leave, got %+v", left)
	}
}

func TestDuelIDsUniqueAcrossRestarts(t *testing.T) {
	// Simulates two fresh processes: duel IDs must never collide, because the
	// reward ledger idempotency key embeds the duel ID (CR-10 bug class).
	ledger := economy.NewMemoryLedger()
	svc1 := NewService(ledger)
	svc2 := NewService(ledger)
	ctx := context.Background()

	d1, _, err := svc1.StartDuel(ctx, "stu-1", 1, "Asia/Jakarta", "en", "RANDOM", "")
	if err != nil {
		t.Fatalf("StartDuel svc1: %v", err)
	}
	d2, _, err := svc2.StartDuel(ctx, "stu-1", 1, "Asia/Jakarta", "en", "RANDOM", "")
	if err != nil {
		t.Fatalf("StartDuel svc2: %v", err)
	}
	if d1.ID == d2.ID {
		t.Fatalf("duel ID reused across restarts: %s", d1.ID)
	}
	suffix := strings.TrimPrefix(d1.ID, "duel-")
	if len(suffix) != 16 {
		t.Fatalf("duel ID should be duel-<16 hex chars>, got %q", d1.ID)
	}

	l1 := svc1.GetLobby(ctx, "stu-9", 1, "Asia/Jakarta", "en")
	l2 := svc2.GetLobby(ctx, "stu-9", 1, "Asia/Jakarta", "en")
	if len(l1.Incoming) != 1 || len(l2.Incoming) != 1 {
		t.Fatalf("expected one incoming duel per lobby, got %d and %d", len(l1.Incoming), len(l2.Incoming))
	}
	if l1.Incoming[0].ID == l2.Incoming[0].ID {
		t.Fatalf("incoming duel ID reused across restarts: %s", l1.Incoming[0].ID)
	}
}

// newSocialTestStudent creates an isolated grade-1 test student (seed pilot1
// must exist in the database first) and removes it when the test ends.
func newSocialTestStudent(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	var studentID string
	if err := pool.QueryRow(context.Background(), `
		INSERT INTO student_profiles (parent_account_id, login_id, password_hash, first_name, masked_display_name, grade, crew_code)
		SELECT parent_account_id, 'social-test-' || gen_random_uuid(), password_hash, 'Test Pilot', 'Test Pilot', 1,
			left(replace(gen_random_uuid()::text, '-', ''), 9)
		FROM student_profiles WHERE login_id = 'pilot1'
		RETURNING id::text
	`).Scan(&studentID); err != nil {
		t.Fatalf("create isolated test student (seed pilot1 first): %v", err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), `DELETE FROM shared_state WHERE scope=$1 AND (key=$2 OR (key LIKE 'code:%' AND payload->'members' @> $3::jsonb))`, crewScope, memberKeyPrefix+studentID, `[{"studentId":"`+studentID+`"}]`); err != nil {
			t.Error(err)
		}
		if _, err := pool.Exec(context.Background(), `DELETE FROM student_profiles WHERE id = $1::uuid`, studentID); err != nil {
			t.Error(err)
		}
	})
	return studentID
}

// TestStarDuelsPersistAcrossRestartPG runs only with TEST_DATABASE_URL set:
// it verifies duel records, completed history, the durable energy counter,
// and the reward ledger behave consistently across a simulated restart.
func TestStarDuelsPersistAcrossRestartPG(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping PostgreSQL duel persistence test")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	t.Cleanup(pool.Close)
	studentID := newSocialTestStudent(t, pool)
	ledger := economy.NewPGLedger(pool)

	// Fresh students have no wallet row until the ledger creates one on the
	// first mutation; start from a known 0-SP balance.
	if _, err := pool.Exec(ctx, `INSERT INTO student_wallets (student_id) VALUES ($1::uuid) ON CONFLICT (student_id) DO NOTHING`, studentID); err != nil {
		t.Fatalf("init wallet row: %v", err)
	}

	var spBefore int64
	if err := pool.QueryRow(ctx, `SELECT total_sp FROM student_wallets WHERE student_id = $1::uuid`, studentID).Scan(&spBefore); err != nil {
		t.Fatalf("read wallet: %v", err)
	}

	svc1 := NewServiceWithPG(pool, ledger)
	fixedNow := time.Date(2026, 11, 28, 10, 0, 0, 0, time.UTC)
	svc1.SetClockForTest(func() time.Time { return fixedNow })
	lobby1 := svc1.GetLobby(ctx, studentID, 1, "Asia/Jakarta", "en")
	if len(lobby1.Incoming) != 1 {
		t.Fatalf("expected 1 incoming duel, got %d", len(lobby1.Incoming))
	}
	incomingID := lobby1.Incoming[0].ID

	outgoing, _, err := svc1.StartDuel(ctx, studentID, 1, "Asia/Jakarta", "en", "RANDOM", "")
	if err != nil {
		t.Fatalf("StartDuel: %v", err)
	}
	if _, err := svc1.AcceptIncoming(ctx, studentID, incomingID, "Asia/Jakarta", "en"); err != nil {
		t.Fatalf("AcceptIncoming: %v", err)
	}
	res, err := svc1.SubmitDuel(ctx, studentID, incomingID, "en", map[int]map[string]string{})
	if err != nil || res.Status != "COMPLETED" {
		t.Fatalf("SubmitDuel: err=%v status=%s", err, res.Status)
	}

	// Fresh process = simulated restart.
	svc2 := NewServiceWithPG(pool, ledger)
	svc2.SetClockForTest(func() time.Time { return fixedNow })
	lobby2 := svc2.GetLobby(ctx, studentID, 1, "Asia/Jakarta", "en")
	completedFound := false
	for _, h := range lobby2.History {
		if h.ID == incomingID && h.Outcome != "" {
			completedFound = true
		}
	}
	if !completedFound {
		t.Fatalf("completed duel missing from history after restart: %+v", lobby2.History)
	}
	if lobby2.EnergyLeft != DailyEnergyMax-2 {
		t.Fatalf("durable energy counter lost across restart: left=%d want=%d", lobby2.EnergyLeft, DailyEnergyMax-2)
	}

	// The accepted duel that was never submitted can still be graded.
	res2, err := svc2.SubmitDuel(ctx, studentID, outgoing.ID, "en", map[int]map[string]string{})
	if err != nil || res2.Status != "COMPLETED" {
		t.Fatalf("SubmitDuel after restart: err=%v status=%s", err, res2.Status)
	}

	// Replay is idempotent and awards nothing extra.
	if _, err := svc2.SubmitDuel(ctx, studentID, incomingID, "en", map[int]map[string]string{}); err != nil {
		t.Fatalf("replay SubmitDuel: %v", err)
	}
	var spAfter int64
	if err := pool.QueryRow(ctx, `SELECT total_sp FROM student_wallets WHERE student_id = $1::uuid`, studentID).Scan(&spAfter); err != nil {
		t.Fatalf("read wallet: %v", err)
	}
	if want := spBefore + DuelLossSP*2; spAfter != want {
		t.Fatalf("expected exactly two duel rewards (%d), wallet went %d -> %d", want, spBefore, spAfter)
	}
}

func TestDuelPersistenceRetainsActiveBeyondHistoryCapPG(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	studentID := newSocialTestStudent(t, pool)
	svc := NewServiceWithPG(pool, nil)
	for i := 0; i < duelsStateCap+2; i++ {
		id := fmt.Sprintf("completed-%d", i)
		svc.duels[id] = &duelInternal{studentID: studentID, record: DuelRecord{ID: id, Status: "COMPLETED"}}
		svc.studentDuels[studentID] = append(svc.studentDuels[studentID], id)
	}
	for _, status := range []string{"PENDING", "IN_PROGRESS"} {
		id := "active-" + status
		svc.duels[id] = &duelInternal{studentID: studentID, record: DuelRecord{ID: id, Status: status}}
		svc.studentDuels[studentID] = append(svc.studentDuels[studentID], id)
	}
	svc.persistDuels(ctx, studentID)
	reloaded := NewServiceWithPG(pool, nil)
	reloaded.hydrateDuels(ctx, studentID)
	if got := len(reloaded.studentDuels[studentID]); got != duelsStateCap+2 {
		t.Fatalf("got %d retained duels", got)
	}
	for _, status := range []string{"PENDING", "IN_PROGRESS"} {
		if reloaded.duels["active-"+status] == nil {
			t.Fatalf("lost %s duel", status)
		}
	}
}

// TestStarCrewPersistsAcrossRestartPG verifies crew codes, rosters, and
// membership survive a restart.
func TestStarCrewPersistsAcrossRestartPG(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping PostgreSQL crew persistence test")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	t.Cleanup(pool.Close)
	pilotA := newSocialTestStudent(t, pool)
	pilotB := newSocialTestStudent(t, pool)

	svc1 := NewServiceWithPG(pool, economy.NewPGLedger(pool))
	crewA := svc1.GetOrCreateCrew(ctx, pilotA, "Test Pilot", 1)
	t.Cleanup(func() {
		if _, err := pool.Exec(ctx, `DELETE FROM shared_state WHERE scope=$1 AND key=$2`, crewScope, crewKeyPrefix+crewA.CrewCode); err != nil {
			t.Error(err)
		}
	})

	// A second instance joins via the code — it must find the durable roster.
	svc2 := NewServiceWithPG(pool, economy.NewPGLedger(pool))
	joined, err := svc2.JoinCrew(ctx, pilotB, "Test Pilot", 1, crewA.CrewCode)
	if err != nil {
		t.Fatalf("JoinCrew from fresh instance: %v", err)
	}
	if len(joined.Members) != 2 {
		t.Fatalf("expected 2 members after join, got %d", len(joined.Members))
	}

	// Fresh process for pilot A: same crew code, roster intact.
	svc3 := NewServiceWithPG(pool, economy.NewPGLedger(pool))
	crewA2 := svc3.GetOrCreateCrew(ctx, pilotA, "Test Pilot", 1)
	if crewA2.CrewCode != crewA.CrewCode {
		t.Fatalf("crew code lost across restart: %s -> %s", crewA.CrewCode, crewA2.CrewCode)
	}
	if len(crewA2.Members) != 2 {
		t.Fatalf("crew roster lost across restart: %d members", len(crewA2.Members))
	}

	left := svc3.LeaveCrew(ctx, pilotB, "Test Pilot", 1)
	if len(left.Members) != 1 {
		t.Fatalf("expected fresh personal crew after leave, got %d members", len(left.Members))
	}
	crewA3 := svc3.GetOrCreateCrew(ctx, pilotA, "Test Pilot", 1)
	if len(crewA3.Members) != 1 {
		t.Fatalf("expected 1 member after pilot B left, got %d", len(crewA3.Members))
	}
}
