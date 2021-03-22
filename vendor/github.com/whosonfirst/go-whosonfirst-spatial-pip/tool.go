package pip

import (
	"context"
	"fmt"
	"github.com/paulmach/orb"
	"github.com/paulmach/orb/geojson"
	"github.com/sfomuseum/go-sfomuseum-mapshaper"
	"github.com/skelterjohn/geom"
	"github.com/tidwall/gjson"
	"github.com/whosonfirst/go-reader"
	"github.com/whosonfirst/go-whosonfirst-export/v2"
	"github.com/whosonfirst/go-whosonfirst-geojson-v2/properties/whosonfirst"
	"github.com/whosonfirst/go-whosonfirst-placetypes"
	wof_reader "github.com/whosonfirst/go-whosonfirst-reader"
	"github.com/whosonfirst/go-whosonfirst-spatial/database"
	"github.com/whosonfirst/go-whosonfirst-spatial/filter"
	"github.com/whosonfirst/go-whosonfirst-spr/v2"
	"strconv"
)

type PointInPolygonTool struct {
	Database  database.SpatialDatabase
	Mapshaper *mapshaper.Client
}

type PointInPolygonToolUpdateCallback func(context.Context, reader.Reader, spr.StandardPlacesResult) (map[string]interface{}, error)

func DefaultPointInPolygonToolUpdateCallback() PointInPolygonToolUpdateCallback {

	fn := func(ctx context.Context, r reader.Reader, parent_spr spr.StandardPlacesResult) (map[string]interface{}, error) {

		to_update := make(map[string]interface{})

		if parent_spr == nil {

			to_update = map[string]interface{}{
				"properties.wof:parent_id": -1,
			}

		} else {

			parent_id, err := strconv.ParseInt(parent_spr.Id(), 10, 64)

			if err != nil {
				return nil, err
			}

			parent_f, err := wof_reader.LoadFeatureFromID(ctx, r, parent_id)

			if err != nil {
				return nil, err
			}

			parent_hierarchy := whosonfirst.Hierarchies(parent_f)
			parent_country := whosonfirst.Country(parent_f)

			to_update = map[string]interface{}{
				"properties.wof:parent_id": parent_id,
				"properties.wof:country":   parent_country,
				"properties.wof:hierarchy": parent_hierarchy,
			}
		}

		return to_update, nil
	}

	return fn
}

func NewPointInPolygonTool(ctx context.Context, spatial_db database.SpatialDatabase, ms_client *mapshaper.Client) (*PointInPolygonTool, error) {

	t := &PointInPolygonTool{
		Database:  spatial_db,
		Mapshaper: ms_client,
	}

	return t, nil
}

func (t *PointInPolygonTool) PointInPolygonAndUpdate(ctx context.Context, inputs *filter.SPRInputs, results_cb FilterSPRResultsFunc, update_cb PointInPolygonToolUpdateCallback, body []byte) ([]byte, error) {

	possible, err := t.PointInPolygon(ctx, inputs, body)

	if err != nil {
		return nil, err
	}

	parent_spr, err := results_cb(ctx, t.Database, body, possible)

	if err != nil {
		return nil, err
	}

	to_assign, err := update_cb(ctx, t.Database, parent_spr)

	if err != nil {
		return nil, err
	}

	if to_assign != nil {

		body, err = export.AssignProperties(ctx, body, to_assign)

		if err != nil {
			return nil, err
		}
	}

	return body, nil
}

func (t *PointInPolygonTool) PointInPolygon(ctx context.Context, inputs *filter.SPRInputs, body []byte) ([]spr.StandardPlacesResult, error) {

	pt_rsp := gjson.GetBytes(body, "properties.wof:placetype")

	if !pt_rsp.Exists() {
		return nil, fmt.Errorf("Missing 'wof:placetype' property")
	}

	pt_str := pt_rsp.String()

	pt, err := placetypes.GetPlacetypeByName(pt_str)

	if err != nil {
		return nil, fmt.Errorf("Failed to create new placetype for '%s', %v", pt_str, err)
	}

	roles := []string{
		"common",
		"optional",
		"common_optional",
	}

	ancestors := placetypes.AncestorsForRoles(pt, roles)

	centroid, err := t.PointInPolygonCentroid(ctx, body)

	if err != nil {
		return nil, err
	}

	lon := centroid.X()
	lat := centroid.Y()

	// Start PIP-ing the list of ancestors - stop at the first match

	possible := make([]spr.StandardPlacesResult, 0)

	for _, a := range ancestors {

		coord := &geom.Coord{
			X: lon,
			Y: lat,
		}

		inputs.Placetypes = []string{a.Name}

		spr_filter, err := filter.NewSPRFilterFromInputs(inputs)

		if err != nil {
			return nil, fmt.Errorf("Failed to create SPR filter from input, %v", err)
		}

		rsp, err := t.Database.PointInPolygon(ctx, coord, spr_filter)

		if err != nil {
			return nil, fmt.Errorf("Failed to point in polygon for %v, %v", coord, err)
		}

		// This should never happen...

		if rsp == nil {
			return nil, fmt.Errorf("Failed to point in polygon for %v, null response", coord)
		}

		results := rsp.Results()

		if len(results) == 0 {
			continue
		}

		possible = results
		break
	}

	return possible, nil
}

func (t *PointInPolygonTool) PointInPolygonCentroid(ctx context.Context, body []byte) (*orb.Point, error) {

	f, err := geojson.UnmarshalFeature(body)

	if err != nil {
		return nil, err
	}

	// First see whether there are exsiting reverse-geocoding properties
	// that we can use

	props := f.Properties

	to_try := []string{
		"reversegeo",
		"lbl",
		"mps",
	}

	for _, prefix := range to_try {

		key_lat := fmt.Sprintf("%s:latitude", prefix)
		key_lon := fmt.Sprintf("%s:longitude", prefix)

		lat, ok_lat := props[key_lat]
		lon, ok_lon := props[key_lon]

		if !ok_lat || ok_lon {
			continue
		}

		pt := &orb.Point{
			lat.(float64),
			lon.(float64),
		}

		return pt, nil
	}

	// Next see what kind of feature we are working with

	var candidate *geojson.Feature

	geojson_type := f.Geometry.GeoJSONType()

	switch geojson_type {
	case "Point":
		candidate = f
	case "MultiPoint":

		// not at all clear this is the best way to deal with things
		// (20210204/thisisaaronland)

		bound := f.Geometry.Bound()
		pt := bound.Center()

		candidate = geojson.NewFeature(pt)

	case "Polygon", "MultiPolygon":

		if t.Mapshaper == nil {

			bound := f.Geometry.Bound()
			pt := bound.Center()

			candidate = geojson.NewFeature(pt)

		} else {

			// this is not great but it's also not hard and making
			// the "perfect" mapshaper interface is yak-shaving right
			// now (20210204/thisisaaronland)

			fc := geojson.NewFeatureCollection()
			fc.Append(f)

			fc, err := t.Mapshaper.AppendCentroids(ctx, fc)

			if err != nil {
				return nil, fmt.Errorf("Failed to append centroids, %v", err)
			}

			f = fc.Features[0]

			candidate = geojson.NewFeature(f.Geometry)

			lat, lat_ok := f.Properties["mps:latitude"]
			lon, lon_ok := f.Properties["mps:longitude"]

			if lat_ok && lon_ok {

				pt := orb.Point{
					lat.(float64),
					lon.(float64),
				}

				candidate = geojson.NewFeature(pt)
			}
		}

	default:
		return nil, fmt.Errorf("Unsupported type '%s'", t)
	}

	pt := candidate.Geometry.(orb.Point)
	return &pt, nil
}
