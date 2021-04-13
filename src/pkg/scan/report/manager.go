// Copyright Project Harbor Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package report

import (
	"context"
	"time"

	"github.com/goharbor/harbor/src/lib/errors"
	"github.com/goharbor/harbor/src/lib/q"
	"github.com/goharbor/harbor/src/pkg/scan/dao/scan"
	"github.com/google/uuid"
)

// Manager is used to manage the scan reports.
type Manager interface {
	// Create a new report record.
	//
	//  Arguments:
	//    ctx context.Context : the context for this method
	//    r *scan.Report : report model to be created
	//
	//  Returns:
	//    string : uuid of the new report
	//    error  : non nil error if any errors occurred
	//
	Create(ctx context.Context, r *scan.Report) (string, error)

	// Delete delete report by uuid
	//
	//  Arguments:
	//    ctx context.Context : the context for this method
	//    uuid string : uuid of the report to delete
	//
	//  Returns:
	//    error  : non nil error if any errors occurred
	//
	Delete(ctx context.Context, uuid string) error

	// Update the report data (with JSON format) of the given report.
	//
	//  Arguments:
	//    ctx context.Context : the context for this method
	//    uuid string    : uuid to identify the report
	//    report string  : report JSON data
	//
	//  Returns:
	//    error  : non nil error if any errors occurred
	//
	UpdateReportData(ctx context.Context, uuid string, report string) error

	// Get the reports for the given digest by other properties.
	//
	//  Arguments:
	//    ctx context.Context : the context for this method
	//    digest string           : digest of the artifact
	//    registrationUUID string : [optional] the report generated by which registration.
	//                              If it is empty, reports by all the registrations are retrieved.
	//    mimeTypes []string      : [optional] mime types of the reports requiring
	//                              If empty array is specified, reports with all the supported mimes are retrieved.
	//
	//  Returns:
	//    []*scan.Report : report list
	//    error          : non nil error if any errors occurred
	GetBy(ctx context.Context, digest string, registrationUUID string, mimeTypes []string) ([]*scan.Report, error)

	// Delete the reports related with the specified digests (one or more...)
	//
	//  Arguments:
	//    ctx context.Context : the context for this method
	//    digests ...string : specify one or more digests whose reports will be deleted
	//
	//  Returns:
	//    error        : non nil error if any errors occurred
	DeleteByDigests(ctx context.Context, digests ...string) error

	// List reports according to the query
	//
	//  Arguments:
	//    ctx context.Context : the context for this method
	//    query *q.Query : the query to list the reports
	//
	//  Returns:
	//    []*scan.Report : report list
	//    error        : non nil error if any errors occurred
	List(ctx context.Context, query *q.Query) ([]*scan.Report, error)
}

const (
	reportTimeout = 1 * time.Hour
)

// basicManager is a default implementation of report manager.
type basicManager struct {
	dao     scan.DAO
	vulnDao scan.VulnerabilityRecordDao
}

// NewManager news basic manager.
func NewManager() Manager {
	return &basicManager{
		dao:     scan.New(),
		vulnDao: scan.NewVulnerabilityRecordDao(),
	}
}

// Create ...
func (bm *basicManager) Create(ctx context.Context, r *scan.Report) (string, error) {
	// Validate report object
	if r == nil {
		return "", errors.New("nil scan report object")
	}

	if len(r.Digest) == 0 || len(r.RegistrationUUID) == 0 || len(r.MimeType) == 0 {
		return "", errors.New("malformed scan report object")
	}

	r.UUID = uuid.New().String()

	// Insert
	if _, err := bm.dao.Create(ctx, r); err != nil {
		return "", err
	}

	return r.UUID, nil
}

func (bm *basicManager) Delete(ctx context.Context, uuid string) error {
	_, err := bm.vulnDao.DeleteForReport(ctx, uuid)
	if err != nil {
		return err
	}
	query := q.Query{Keywords: q.KeyWords{"uuid": uuid}}
	count, err := bm.dao.DeleteMany(ctx, query)
	if err != nil {
		return err
	}

	if count == 0 {
		return errors.Errorf("no report with uuid %s deleted", uuid)
	}

	return nil
}

// GetBy ...
func (bm *basicManager) GetBy(ctx context.Context, digest string, registrationUUID string, mimeTypes []string) ([]*scan.Report, error) {
	if len(digest) == 0 {
		return nil, errors.New("empty digest to get report data")
	}

	kws := make(map[string]interface{})
	kws["digest"] = digest
	if len(registrationUUID) > 0 {
		kws["registration_uuid"] = registrationUUID
	}
	if len(mimeTypes) > 0 {
		kws["mime_type__in"] = mimeTypes
	}
	// Query all
	query := &q.Query{
		PageNumber: 0,
		Keywords:   kws,
	}

	return bm.dao.List(ctx, query)
}

// UpdateReportData ...
func (bm *basicManager) UpdateReportData(ctx context.Context, uuid string, report string) error {
	if len(uuid) == 0 {
		return errors.New("missing uuid")
	}

	if len(report) == 0 {
		return errors.New("missing report JSON data")
	}

	return bm.dao.UpdateReportData(ctx, uuid, report)
}

// DeleteByDigests ...
func (bm *basicManager) DeleteByDigests(ctx context.Context, digests ...string) error {
	if len(digests) == 0 {
		// Nothing to do
		return nil
	}

	// delete the vulnerability records to the report UUID mapping for the digests
	// provided
	_, err := bm.vulnDao.DeleteForDigests(ctx, digests...)

	if err != nil {
		return err
	}
	var ol q.OrList
	for _, digest := range digests {
		ol.Values = append(ol.Values, digest)
	}

	query := q.Query{Keywords: q.KeyWords{"digest": &ol}}
	_, err = bm.dao.DeleteMany(ctx, query)
	return err
}

func (bm *basicManager) List(ctx context.Context, query *q.Query) ([]*scan.Report, error) {
	return bm.dao.List(ctx, query)
}
