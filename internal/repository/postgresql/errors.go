package postgresql

import "fmt"

func createQueryError(err error) error {
	return fmt.Errorf("failed to create query: %w", err)
}

func executeQueryError(err error) error {
	return fmt.Errorf("failed to execute query: %w", err)
}

func scanRowError(err error) error {
	return fmt.Errorf("failed to scan row: %w", err)
}

func collectRowsError(err error) error {
	return fmt.Errorf("failed to collect rows: %w", err)
}
