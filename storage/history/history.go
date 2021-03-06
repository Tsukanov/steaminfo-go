package history

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	"bitbucket.org/kardianos/osext"
	_ "github.com/mattn/go-sqlite3"
	"github.com/tsukanov/steamhistory/storage/apps"
)

const (
	UsageHistoryLocation = "data/history" // Directory which contains databases with usage history
)

// OpenAppUsageDB opens database with usage history for a specified application
// and, if successful, returns a reference to it.
func OpenAppUsageDB(appID int) (*sql.DB, error) {
	exeloc, err := osext.ExecutableFolder()
	if err != nil {
		return nil, err
	}
	err = os.MkdirAll(exeloc+UsageHistoryLocation, 0774)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite3", fmt.Sprintf("%s%s/%d.db", exeloc, UsageHistoryLocation, appID))
	if err != nil {
		return nil, err
	}
	// TODO: Check if file exists before attempting to create a table
	sqlInitDB := `
			CREATE TABLE IF NOT EXISTS records (
				time DATETIME NOT NULL PRIMARY KEY,
				count INTEGER NOT NULL
			);
			`
	_, err = db.Exec(sqlInitDB)
	if err != nil {
		return nil, err
	}
	return db, nil
}

// RemoveAppUsageDB removes database with usage history for a specified application.
func RemoveAppUsageDB(appID int) error {
	exeloc, err := osext.ExecutableFolder()
	if err != nil {
		return err
	}
	return os.Remove(fmt.Sprintf("%s%s/%d.db", exeloc, UsageHistoryLocation, appID))
}

// MakeUsageRecord adds a record with current numer of users for a specified
// application.
func MakeUsageRecord(appID int, userCount int, currentTime time.Time) error {
	db, err := OpenAppUsageDB(appID)
	if err != nil {
		return err
	}
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare("INSERT INTO records (time, count) VALUES (?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()
	_, err = stmt.Exec(currentTime.Unix(), userCount)
	if err != nil {
		return err
	}
	tx.Commit()
	return nil
}

// AllUsageHistory returns usage data for specified application as a collection
// of two integers. First is a time, second - number of users.
func AllUsageHistory(appID int) (history [][2]int64, err error) {
	db, err := OpenAppUsageDB(appID)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query("SELECT time, count FROM records ORDER BY time")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var record [2]int64
		var t time.Time
		err := rows.Scan(&t, &record[1])
		if err != nil {
			return nil, err
		}
		record[0] = t.Unix() * 1000 // For JavaScript
		history = append(history, record)
	}
	rows.Close()
	return history, nil
}

func cleanup(appID int) {
	db, err := OpenAppUsageDB(appID)
	if err != nil {
		log.Println(err)
		return
	}
	defer db.Close()
	_, err = db.Exec("DELETE FROM records WHERE count=0")
	if err != nil {
		log.Println(err)
		return
	}
}

// HistoryCleanup removes all records with 0 value for all usable apps.
// This fixes an issue when Steam API sometimes returns 0 value.
func HistoryCleanup() error {
	applications, err := apps.AllUsableApps()
	if err != nil {
		return err
	}
	for _, app := range applications {
		cleanup(app.ID)
	}
	return nil
}

// GetPeakBetween returns peak and it's time for a specified application
// in between specified time period.
func GetPeakBetween(start time.Time, end time.Time, appID int) (count int, time time.Time, err error) {
	db, err := OpenAppUsageDB(appID)
	if err != nil {
		return count, time, err
	}
	defer db.Close()
	stmt, err := db.Prepare(`
		SELECT count, time
		FROM records
		WHERE time BETWEEN ? AND ?
		ORDER BY -count
		LIMIT 1
		`)
	if err != nil {
		return count, time, err
	}
	defer stmt.Close()

	err = stmt.QueryRow(start.Unix(), end.Unix()).Scan(&count, &time)
	if err != nil {
		return count, time, err
	}
	return count, time, nil
}
