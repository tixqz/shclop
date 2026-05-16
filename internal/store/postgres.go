package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/mipopov/shclop/internal/domain"
)

type Postgres struct {
	db *sql.DB
}

func NewPostgres(dsn string) (*Postgres, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("connect postgres: %w", err)
	}
	return &Postgres{db: db}, nil
}

func (p *Postgres) CreateAgent(ctx context.Context, ownerID, name string) (domain.Agent, error) {
	id, err := newID()
	if err != nil {
		return domain.Agent{}, err
	}
	agent := domain.Agent{ID: id, OwnerID: ownerID, Name: name, State: "idle", CreatedAt: time.Now().UTC()}
	_, err = p.db.ExecContext(ctx, `
		insert into agents (id, owner_id, name, state, created_at)
		values ($1, $2, $3, $4, $5)
	`, agent.ID, agent.OwnerID, agent.Name, agent.State, agent.CreatedAt)
	if err != nil {
		return domain.Agent{}, err
	}
	return agent, nil
}

func (p *Postgres) ListAgents(ctx context.Context, ownerID string) ([]domain.Agent, error) {
	rows, err := p.db.QueryContext(ctx, `
		select id, owner_id, name, state, created_at
		from agents
		where owner_id = $1
		order by created_at asc, id asc
	`, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var agents []domain.Agent
	for rows.Next() {
		var agent domain.Agent
		if err := rows.Scan(&agent.ID, &agent.OwnerID, &agent.Name, &agent.State, &agent.CreatedAt); err != nil {
			return nil, err
		}
		agents = append(agents, agent)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return agents, nil
}

func (p *Postgres) GetAgent(ctx context.Context, agentID string) (domain.Agent, error) {
	var agent domain.Agent
	err := p.db.QueryRowContext(ctx, `
		select id, owner_id, name, state, created_at
		from agents
		where id = $1
	`, agentID).Scan(&agent.ID, &agent.OwnerID, &agent.Name, &agent.State, &agent.CreatedAt)
	if err == sql.ErrNoRows {
		return domain.Agent{}, ErrNotFound
	}
	if err != nil {
		return domain.Agent{}, err
	}
	return agent, nil
}

func (p *Postgres) UpdateAgentState(ctx context.Context, agentID, state string) (domain.Agent, error) {
	var agent domain.Agent
	err := p.db.QueryRowContext(ctx, `
		update agents
		set state = $2
		where id = $1
		returning id, owner_id, name, state, created_at
	`, agentID, state).Scan(&agent.ID, &agent.OwnerID, &agent.Name, &agent.State, &agent.CreatedAt)
	if err == sql.ErrNoRows {
		return domain.Agent{}, ErrNotFound
	}
	if err != nil {
		return domain.Agent{}, err
	}
	return agent, nil
}

func (p *Postgres) Close() error {
	return p.db.Close()
}
