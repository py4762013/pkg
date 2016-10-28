package binlogsync

import (
	"context"
	"time"

	"github.com/corestoreio/csfw/log"
	"github.com/corestoreio/csfw/util/errors"
	"github.com/siddontang/go-mysql/mysql"
	"github.com/siddontang/go-mysql/replication"
)

const (
	UpdateAction = "update"
	InsertAction = "insert"
	DeleteAction = "delete"
)

func (c *Canal) startSyncBinlog(ctxArg context.Context) error {
	pos := c.masterStatus

	if c.Log.IsInfo() {
		c.Log.Info("[binlogsync] Start syncing of binlog", log.Stringer("position", pos))
	}

	fixMePos := mysql.Position{
		Name: pos.File,
		Pos:  uint32(pos.Position),
	}

	s, err := c.syncer.StartSync(fixMePos)
	if err != nil {
		return errors.NewFatalf("[binlogsync] Start sync replication at %s error %v", pos, err)
	}

	timeout := time.Second
	for {
		ctx, cancel := context.WithTimeout(ctxArg, 2*time.Second)
		ev, err := s.GetEvent(ctx)
		cancel()

		if err == context.DeadlineExceeded {
			timeout = 2 * timeout
			continue
		}
		if err != nil {
			return errors.Wrap(err, "[binlogsync] startSyncBinlog.GetEvent")
		}

		timeout = time.Second

		//next binlog pos
		pos.Position = uint(ev.Header.LogPos)

		switch e := ev.Event.(type) {
		case *replication.RotateEvent:
			if err := c.flushEventHandlers(ctxArg); err != nil {
				// todo maybe better err handling ...
				return errors.Wrap(err, "[binlogsync] startSyncBinlog.flushEventHandlers")
			}
			pos.File = string(e.NextLogName)
			pos.Position = uint(e.Position)
			// r.ev <- pos

			if c.Log.IsInfo() {
				c.Log.Info("[binlogsync] Rotate binlog to a new position", log.Stringer("position", pos))
			}

		case *replication.RowsEvent:
			// we only focus row based event
			if err = c.handleRowsEvent(ctxArg, ev); err != nil {
				if c.Log.IsInfo() {
					c.Log.Info("[binlogsync] Rotate binlog to a new position", log.Err(err), log.Stringer("position", pos))
				}
				return errors.Wrap(err, "[binlogsync] handleRowsEvent")
			}
		case *replication.TableMapEvent:
			continue
			//default:
			//	fmt.Printf("%#v\n\n", e)
		}

		c.masterUpdate(pos.File, pos.Position)
		if err := c.masterSave(); err != nil {
			c.Log.Info("[binlogsync] startSyncBinlog: Failed to save master position", log.Err(err), log.Stringer("position", pos))
		}
	}

	return nil
}

func (c *Canal) handleRowsEvent(ctx context.Context, e *replication.BinlogEvent) error {
	ev, ok := e.Event.(*replication.RowsEvent)
	if !ok {
		return errors.NewFatalf("[binlogsync] handleRowsEvent: Failed to cast to *replication.RowsEvent type")
	}

	// Caveat: table may be altered at runtime.

	if in := string(ev.Table.Schema); c.dsn.DBName != in {
		if c.Log.IsDebug() {
			c.Log.Debug("[binlogsync] Skipping database", log.String("database_have", in), log.String("database_want", c.dsn.DBName), log.Int("table_id", int(ev.TableID)))
		}
		return nil
	}

	table := string(ev.Table.Table)

	t, err := c.FindTable(ctx, int(ev.TableID), table)
	if err != nil {
		return errors.Wrapf(err, "[binlogsync] GetTable %q.%q", c.dsn.DBName, table)
	}
	var action string
	switch e.Header.EventType {
	case replication.WRITE_ROWS_EVENTv1, replication.WRITE_ROWS_EVENTv2:
		action = InsertAction
	case replication.DELETE_ROWS_EVENTv1, replication.DELETE_ROWS_EVENTv2:
		action = DeleteAction
	case replication.UPDATE_ROWS_EVENTv1, replication.UPDATE_ROWS_EVENTv2:
		action = UpdateAction
	default:
		return errors.NewNotSupportedf("[binlogsync] EventType %v not yet supported", e.Header.EventType)
	}
	return c.travelRowsEventHandler(ctx, action, t, ev.Rows)
}

// todo: implement when needed
//func (c *Canal) WaitUntilPos(pos mysql.Position, timeout int) error {
//	if timeout <= 0 {
//		timeout = 60
//	}
//
//	timer := time.NewTimer(time.Duration(timeout) * time.Second)
//	for {
//		select {
//		case <-timer.C:
//			return errors.NewTimeoutf("[binlogsync] WaitUntilPos wait position %v err", pos)
//		default:
//			if c.masterPos.Compare(pos) >= 0 {
//				return nil
//			} else {
//				time.Sleep(100 * time.Millisecond)
//			}
//		}
//	}
//
//	return nil
//}
//
//func (c *Canal) CatchMasterPos(timeout int) error {
//	rr, err := c.Execute("SHOW MASTER STATUS")
//	if err != nil {
//		return errors.Wrap(err, "[binlogsync] CatchMasterPos")
//	}
//
//	name, _ := rr.GetString(0, 0)
//	pos, _ := rr.GetInt(0, 1)
//
//	return c.WaitUntilPos(mysql.Position{Name: name, Pos: uint32(pos)}, timeout)
//}