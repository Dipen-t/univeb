package repository

import (
	
	"univibe/conductor/models" // <--- Make sure this matches your go.mod name (e.g. module univibe/conductor)

	"gorm.io/gorm"
)

type QueueRepo struct {
	DB *gorm.DB
}

func NewQueueRepo(db *gorm.DB) *QueueRepo {
	return &QueueRepo{DB: db}
}

// 1. GET THE ROOM
func (r *QueueRepo) GetRoom(code string) (*models.Room, error) {
	var room models.Room
	err := r.DB.Where("code = ?", code).First(&room).Error
	return &room, err
}

// 2. SET PLAY/PAUSE STATE
func (r *QueueRepo) SetRoomState(roomID string, isPlaying bool) {
	r.DB.Model(&models.Room{}).Where("id = ?", roomID).Update("is_playing", isPlaying)
}

// 3. GET CURRENTLY PLAYING
func (r *QueueRepo) GetCurrent(roomID string) (*models.QueueItem, error) {
	var item models.QueueItem
	err := r.DB.Preload("Song").
		Where("room_id = ? AND status = ?", roomID, "playing").
		First(&item).Error
	return &item, err
}

// 4. FIND NEXT SONG (Standard Queue)
func (r *QueueRepo) GetNext(roomID string) (*models.QueueItem, error) {
	var item models.QueueItem
	err := r.DB.Preload("Song").
		Where("room_id = ? AND status = ?", roomID, "waiting").
		Order("position asc"). // First in line
		First(&item).Error
	return &item, err
}

// 5. FIND PREVIOUS SONG (History)
func (r *QueueRepo) GetPrevious(roomID string) (*models.QueueItem, error) {
	var item models.QueueItem
	// We find the 'played' song that was updated MOST RECENTLY
	err := r.DB.Preload("Song").
		Where("room_id = ? AND status = ?", roomID, "played").
		Order("updated_at desc"). // Last one to be finished
		First(&item).Error
	return &item, err
}

// 6. MARK SONG AS PLAYED
func (r *QueueRepo) MarkAsPlayed(itemID uint) {
	r.DB.Model(&models.QueueItem{}).Where("id = ?", itemID).
		Updates(map[string]interface{}{
			"status": "played",
			"updated_at": gorm.Expr("NOW()"), // Update timestamp for history sorting
		})
}

// 7. MARK SONG AS WAITING (For putting back in queue)
func (r *QueueRepo) MarkAsWaiting(itemID uint, position int) {
	r.DB.Model(&models.QueueItem{}).Where("id = ?", itemID).
		Updates(map[string]interface{}{
			"status":   "waiting",
			"position": position,
		})
}

// 8. MARK SONG AS PLAYING
func (r *QueueRepo) MarkAsPlaying(itemID uint) {
	r.DB.Model(&models.QueueItem{}).Where("id = ?", itemID).
		Updates(map[string]interface{}{
			"status": "playing",
			"position": 0,
		})
}
// 9. REMOVE ITEM FROM QUEUE
func (r *QueueRepo) RemoveItem(itemID uint) error {
	return r.DB.Delete(&models.QueueItem{}, itemID).Error
}

// 10. GET ALL 
func (r *QueueRepo) GetQueue(roomID string) ([]models.QueueItem, error) {
	var items []models.QueueItem
	err := r.DB.Preload("Song").
		Where("room_id = ?", roomID). // REMOVED "status = waiting"
		Order("id asc").              // Keep original order
		Find(&items).Error
	return items, err
}

// 11. JUMP TO SONG (New Feature)
func (r *QueueRepo) JumpToSong(roomID string, targetItemID uint) error {
	// 1. Mark EVERYTHING as 'played' (Reset state)
	r.DB.Model(&models.QueueItem{}).
		Where("room_id = ?", roomID).
		Update("status", "played")

	// 2. Mark TARGET as 'playing'
	return r.DB.Model(&models.QueueItem{}).
		Where("id = ?", targetItemID).
		Update("status", "playing").Error
	
	// Note: In a complex app, you'd mark subsequent songs as 'waiting', 
	// but for this simple list, 'played' vs 'playing' vs 'waiting' is visual.
	// Logic-wise, 'played' songs are just history.
}
