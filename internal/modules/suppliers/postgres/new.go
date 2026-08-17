package postgres

import (
	"github.com/jackc/pgx/v5/pgxpool"
)

func NewRepository(pool *pgxpool.Pool) *repository { return &repository{pool: pool} }
