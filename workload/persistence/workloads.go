// Copyright 2015 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package persistence

// TODO(ericsnow) Eliminate the mongo-related imports here.

import (
	"github.com/juju/errors"
	"github.com/juju/loggo"
	"github.com/juju/names"
	jujutxn "github.com/juju/txn"
	"gopkg.in/mgo.v2/txn"

	"github.com/juju/juju/workload"
)

var logger = loggo.GetLogger("juju.workload.persistence")

// TODO(ericsnow) Store status in the status collection?

// TODO(ericsnow) Implement persistence using a TXN abstraction (used
// in the business logic) with ops factories available from the
// persistence layer.

// TODO(ericsnow) Move PersistenceBase to the components package?

// PersistenceBase exposes the core persistence functionality needed
// for workloads.
type PersistenceBase interface {
	// One populates doc with the document corresponding to the given
	// ID. Missing documents result in errors.NotFound.
	One(collName, id string, doc interface{}) error
	// All populates docs with the list of the documents corresponding
	// to the provided query.
	All(collName string, query, docs interface{}) error
	// Run runs the transaction generated by the provided factory
	// function. It may be retried several times.
	Run(transactions jujutxn.TransactionSource) error
}

// Persistence exposes the high-level persistence functionality
// related to workloads in Juju.
type Persistence struct {
	st   PersistenceBase
	unit names.UnitTag
}

// NewPersistence builds a new Persistence based on the provided info.
func NewPersistence(st PersistenceBase, unit names.UnitTag) *Persistence {
	return &Persistence{
		st:   st,
		unit: unit,
	}
}

// Track adds records for the workload to persistence. If the workload
// is already there then false gets returned (true if inserted).
// Existing records are not checked for consistency.
func (pp Persistence) Track(id string, info workload.Info) (bool, error) {
	logger.Tracef("insertng %#v", info)

	var okay bool
	var ops []txn.Op
	// TODO(ericsnow) Add unitPersistence.newEnsureAliveOp(pp.unit)?
	ops = append(ops, pp.newInsertWorkloadOps(id, info)...)
	buildTxn := func(attempt int) ([]txn.Op, error) {
		if attempt > 0 {
			okay = false
			return nil, jujutxn.ErrNoOperations
		}
		okay = true
		return ops, nil
	}
	if err := pp.st.Run(buildTxn); err != nil {
		return false, errors.Trace(err)
	}
	return okay, nil
}

// SetStatus updates the raw status for the identified workload in
// persistence. The return value corresponds to whether or not the
// record was found in persistence. Any other problem results in
// an error. The workload is not checked for inconsistent records.
func (pp Persistence) SetStatus(id, status string) (bool, error) {
	logger.Tracef("setting status for %q", id)

	var found bool
	var ops []txn.Op
	// TODO(ericsnow) Add unitPersistence.newEnsureAliveOp(pp.unit)?
	ops = append(ops, pp.newSetRawStatusOps(id, status)...)
	buildTxn := func(attempt int) ([]txn.Op, error) {
		if attempt > 0 {
			found = false
			return nil, jujutxn.ErrNoOperations
		}
		found = true
		return ops, nil
	}
	if err := pp.st.Run(buildTxn); err != nil {
		return false, errors.Trace(err)
	}
	return found, nil
}

// List builds the list of workloads found in persistence which match
// the provided IDs. The lists of IDs with missing records is also
// returned.
func (pp Persistence) List(ids ...string) ([]workload.Info, []string, error) {
	// TODO(ericsnow) Ensure that the unit is Alive?

	workloadDocs, err := pp.workloads(ids)
	if err != nil {
		return nil, nil, errors.Trace(err)
	}

	var results []workload.Info
	var missing []string
	for _, id := range ids {
		w, ok := pp.extractWorkload(id, workloadDocs)
		if !ok {
			missing = append(missing, id)
			continue
		}
		results = append(results, *w)
	}
	return results, missing, nil
}

// ListAll builds the list of all workloads found in persistence.
// Inconsistent records result in errors.NotValid.
func (pp Persistence) ListAll() ([]workload.Info, error) {
	// TODO(ericsnow) Ensure that the unit is Alive?

	workloadDocs, err := pp.allWorkloads()
	if err != nil {
		return nil, errors.Trace(err)
	}

	var results []workload.Info
	for id := range workloadDocs {
		w, _ := pp.extractWorkload(id, workloadDocs)
		results = append(results, *w)
	}
	return results, nil
}

// TODO(ericsnow) Add workloads to state/cleanup.go.

// TODO(ericsnow) How to ensure they are completely removed from state
// (when you factor in status stored in a separate collection)?

// Untrack removes all records associated with the identified workload
// from persistence. Also returned is whether or not the workload was
// found. If the records for the workload are not consistent then
// errors.NotValid is returned.
func (pp Persistence) Untrack(id string) (bool, error) {
	var found bool
	var ops []txn.Op
	// TODO(ericsnow) Add unitPersistence.newEnsureAliveOp(pp.unit)?
	ops = append(ops, pp.newRemoveWorkloadOps(id)...)
	buildTxn := func(attempt int) ([]txn.Op, error) {
		if attempt > 0 {
			found = false
			return nil, jujutxn.ErrNoOperations
		}
		found = true
		return ops, nil
	}
	if err := pp.st.Run(buildTxn); err != nil {
		return false, errors.Trace(err)
	}
	return found, nil
}
