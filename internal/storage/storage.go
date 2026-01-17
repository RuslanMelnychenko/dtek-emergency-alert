package storage

import (
	"dtek-emergency-alert/internal/models"
	"encoding/json"
	"os"
)

type Storage interface {
	Save(data models.SavedInfo) error
	Load() (*models.SavedInfo, error)
}

type fileStorage struct {
	path string
}

func NewFileStorage(path string) Storage {
	return &fileStorage{path: path}
}

func (s *fileStorage) Save(data models.SavedInfo) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, jsonData, 0644)
}

func (s *fileStorage) Load() (*models.SavedInfo, error) {
	if _, err := os.Stat(s.path); os.IsNotExist(err) {
		return nil, nil
	}
	jsonData, err := os.ReadFile(s.path)
	if err != nil {
		return nil, err
	}
	var data models.SavedInfo
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return nil, err
	}
	return &data, nil
}
