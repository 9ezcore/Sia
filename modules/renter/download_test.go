package renter

import (
	"testing"
	"time"
)

// TestClearDownloads tests all the edge cases of the ClearDownloadHistory Method
func TestClearDownloads(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	rt, err := newRenterTester(t.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()

	// Check Clearing individual download from history
	// doesn't exist - before
	length, err := clearDownloadHistory(rt, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(rt.renter.downloadHistory) != length {
		t.Fatal("Download should not have been cleared")
	}
	// doesn't exist - after
	length, err = clearDownloadHistory(rt, 10, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rt.renter.downloadHistory) != length {
		t.Fatal("Download should not have been cleared")
	}
	// doesn't exist - within range
	length, err = clearDownloadHistory(rt, 5, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(rt.renter.downloadHistory) != length {
		t.Fatal("Download should not have been cleared")
	}
	// Remove Last Download
	length, err = clearDownloadHistory(rt, 9, 9)
	if err != nil {
		t.Fatal(err)
	}
	if len(rt.renter.downloadHistory) != length-1 {
		t.Fatal("Download should have been cleared")
	}
	if rt.renter.downloadHistory[length-2].staticStartTime.Unix() == 9 {
		t.Fatal("Download not removed")
	}
	// Remove First Download
	length, err = clearDownloadHistory(rt, 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(rt.renter.downloadHistory) != length-1 {
		t.Fatal("Download should have been cleared")
	}
	if rt.renter.downloadHistory[0].staticStartTime.Unix() == 2 {
		t.Fatal("Download not removed")
	}
	// Remove download from middle of history
	length, err = clearDownloadHistory(rt, 6, 6)
	if err != nil {
		t.Fatal(err)
	}
	if len(rt.renter.downloadHistory) != length-1 {
		t.Fatal("Download should have been cleared")
	}
	for _, d := range rt.renter.downloadHistory {
		if d.staticStartTime.Unix() == 6 {
			t.Fatal("Download not removed")
		}
	}

	// Check Clearing range
	// both exist - first and last
	_, err = clearDownloadHistory(rt, 9, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(rt.renter.downloadHistory) != 0 {
		t.Fatal("Download history should have been cleared")
	}
	// both exist - within range
	_, err = clearDownloadHistory(rt, 8, 3)
	if err != nil {
		t.Fatal(err)
	}
	if !checkDownloadHistory(rt.renter.downloadHistory, []int64{2, 9}) {
		t.Fatal("Download history not cleared as expected")
	}
	// exist - within range and doesn't exist - before
	_, err = clearDownloadHistory(rt, 4, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !checkDownloadHistory(rt.renter.downloadHistory, []int64{6, 8, 9}) {
		t.Fatal("Download history not cleared as expected")
	}
	// exist - within range and doesn't exist - after
	_, err = clearDownloadHistory(rt, 10, 6)
	if err != nil {
		t.Fatal(err)
	}
	if !checkDownloadHistory(rt.renter.downloadHistory, []int64{2, 3, 4}) {
		t.Fatal("Download history not cleared as expected")
	}
	// neither exist - within range and before
	_, err = clearDownloadHistory(rt, 5, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !checkDownloadHistory(rt.renter.downloadHistory, []int64{6, 8, 9}) {
		t.Fatal("Download history not cleared as expected")
	}
	// neither exist - within range and after
	_, err = clearDownloadHistory(rt, 10, 5)
	if err != nil {
		t.Fatal(err)
	}
	if !checkDownloadHistory(rt.renter.downloadHistory, []int64{2, 3, 4}) {
		t.Fatal("Download history not cleared as expected")
	}
	// neither exist - outside
	_, err = clearDownloadHistory(rt, 10, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(rt.renter.downloadHistory) != 0 {
		t.Fatal("Download history should have been cleared")
	}
	// neither exist - inside
	_, err = clearDownloadHistory(rt, 7, 5)
	if err != nil {
		t.Fatal(err)
	}
	if !checkDownloadHistory(rt.renter.downloadHistory, []int64{2, 3, 4, 8, 9}) {
		t.Fatal("Download history not cleared as expected")
	}

	// Check Clear Before
	// exists - within range
	_, err = clearDownloadHistory(rt, 6, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !checkDownloadHistory(rt.renter.downloadHistory, []int64{8, 9}) {
		t.Fatal("Download history not cleared as expected")
	}
	// exists - last
	_, err = clearDownloadHistory(rt, 9, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(rt.renter.downloadHistory) != 0 {
		t.Fatal("Download history should have been cleared")
	}
	// doesn't exist - within range
	_, err = clearDownloadHistory(rt, 7, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !checkDownloadHistory(rt.renter.downloadHistory, []int64{8, 9}) {
		t.Fatal("Download history not cleared as expected")
	}
	// doesn't exist - before
	length, err = clearDownloadHistory(rt, 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(rt.renter.downloadHistory) != length {
		t.Fatal("No downloads should not have been cleared")
	}
	// doesn't exist - after
	_, err = clearDownloadHistory(rt, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(rt.renter.downloadHistory) != 0 {
		t.Fatal("Download history should have been cleared")
	}

	// Check Clear After
	// exists - within range
	_, err = clearDownloadHistory(rt, 0, 6)
	if err != nil {
		t.Fatal(err)
	}
	if !checkDownloadHistory(rt.renter.downloadHistory, []int64{2, 3, 4}) {
		t.Fatal("Download history not cleared as expected")
	}
	// exist - first
	_, err = clearDownloadHistory(rt, 0, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(rt.renter.downloadHistory) != 0 {
		t.Fatal("Download history should have been cleared")
	}
	// doesn't exist - within range
	_, err = clearDownloadHistory(rt, 0, 5)
	if err != nil {
		t.Fatal(err)
	}
	if !checkDownloadHistory(rt.renter.downloadHistory, []int64{2, 3, 4}) {
		t.Fatal("Download history not cleared as expected")
	}
	// doesn't exist - after
	length, err = clearDownloadHistory(rt, 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rt.renter.downloadHistory) != length {
		t.Fatal("No downloads should not have been cleared")
	}
	// doesn't exist - before
	_, err = clearDownloadHistory(rt, 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(rt.renter.downloadHistory) != 0 {
		t.Fatal("Download history should have been cleared")
	}

	// Check clearing entire download history
	_, err = clearDownloadHistory(rt, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(rt.renter.downloadHistory) != 0 {
		t.Fatal("Download History not cleared")
	}
}

// clearDownloadHistory is a helper function for TestClearDownloads, it builds and resets the download
// history of the renter and then calls ClearDownloadHistory and returns the length
// of the original download history
func clearDownloadHistory(rt *renterTester, before, after int) (int, error) {
	// Build/Reset download History
	// Skipping 5 and 7 so there are clear times missing that can
	// be referenced
	rt.renter.downloadHistoryMu.Lock()
	downloads := []*download{}
	for i := 2; i < 10; i++ {
		if i != 5 && i != 7 {
			d := &download{
				staticStartTime: time.Unix(int64(i), 0),
			}
			downloads = append(downloads, d)
		}
	}
	rt.renter.downloadHistory = downloads
	length := len(rt.renter.downloadHistory)
	rt.renter.downloadHistoryMu.Unlock()

	// clear download history
	var beforeTime, afterTime time.Time
	if before != 0 {
		beforeTime = time.Unix(int64(before), 0)
	}
	if after != 0 {
		afterTime = time.Unix(int64(after), 0)
	}
	if err := rt.renter.ClearDownloadHistory(beforeTime, afterTime); err != nil {
		return 0, err
	}
	return length, nil
}

// checkDownloadHistory is a helper function for TestClearDownloads
// it compares the renter's download history against what is expected
// after ClearDownloadHistory is called
func checkDownloadHistory(downloads []*download, check []int64) bool {
	if downloads == nil && check == nil {
		return true
	}
	if downloads == nil || check == nil {
		return false
	}
	if len(downloads) != len(check) {
		return false
	}
	for i := range downloads {
		if downloads[i].staticStartTime.Unix() != check[i] {
			return false
		}
	}
	return true
}
