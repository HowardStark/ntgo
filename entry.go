package ntgo

import (
	"errors"
	"github.com/imdario/mergo"
	"io"
)

const (
	EntryTypeBoolean    EntryType = 0x00
	EntryTypeDouble               = 0x01
	EntryTypeString               = 0x02
	EntryTypeRawData              = 0x03
	EntryTypeBooleanArr           = 0x10
	EntryTypeDoubleArr            = 0x11
	EntryTypeStringArr            = 0x12
	EntryTypeRPCDef               = 0x20
	EntryTypeUndef                = 0xFF

	EntryFlagTemporary  EntryFlag = 0x00
	EntryFlagPersistent           = 0x01
	EntryFlagReserved             = 0xFE
	EntryFlagUndef                = 0xFF
)

const (
	BoolFalse byte = iota
	BoolTrue
)

var (
	ErrEntryTypeCastInvalid = errors.New("entry: could not cast entrytype to type")

	ErrEntryCastInvalid = errors.New("entry: could not cast entryvalue to type")
	ErrEntryNoSuchType  = errors.New("entry: no such type")
	ErrEntryDataInvalid = errors.New("entry: data invalid")

	ErrArrayIndexOutOfBounds = errors.New("entryarray: index out of bounds")
	ErrArrayOutOfSpace       = errors.New("entryarray: no more space")

	ErrEntryFlagNoSuchType = errors.New("entryflag: no such flag")
)

type EntryType byte
type EntryFlag byte

type Entry struct {
	Name     *ValueString
	Type     EntryType
	ID       [2]byte
	Sequence [2]byte
	Flags    EntryFlag
	Value    EntryValue
}

type EntryValue interface {
	GetRaw() []byte
}

type EntryValueArray interface {
	Get(uint8) (EntryValue, error)
	Update(uint8, EntryValue) error
	Add(EntryValue) error
	EntryValue
}

func DecodeEntryFlag(r io.Reader) (EntryFlag, error) {
	flagRaw := make([]byte, 1)
	_, flagErr := r.Read(flagRaw)
	if flagErr != nil {
		return EntryFlagUndef, flagErr
	}
	flag := EntryFlag(flagRaw[0])
	switch flag {
	case EntryFlagPersistent, EntryFlagTemporary, EntryFlagReserved:
		return flag, nil
	default:
		return EntryFlagUndef, ErrEntryFlagNoSuchType
	}
}

func DecodeEntryType(r io.Reader) (EntryType, error) {
	rawType := make([]byte, 1)
	_, readErr := r.Read(rawType)
	if readErr != nil {
		return EntryTypeUndef, readErr
	}
	return EntryType(rawType[0]), nil
}

func DecodeEntryValueAndType(r io.Reader) (value EntryValue, entryType EntryType, err error) {
	entryTypeRaw := make([]byte, 1)
	_, readErr := r.Read(entryTypeRaw)
	if readErr != nil {
		return nil, EntryTypeUndef, readErr
	}
	entryType = EntryType(entryTypeRaw[0])
	value, err = DecodeEntryValue(r, entryType)
	return
}

func DecodeEntryValue(r io.Reader, entryType EntryType) (EntryValue, error) {
	switch entryType {
	case EntryTypeBoolean:
		return DecodeBoolean(r)
	case EntryTypeDouble:
		return DecodeDouble(r)
	case EntryTypeString:
		return DecodeString(r)
	case EntryTypeRawData:
		return DecodeRaw(r)
	case EntryTypeBooleanArr:
		return DecodeBooleanArray(r)
	case EntryTypeDoubleArr:
		return DecodeDoubleArray(r)
	case EntryTypeStringArr:
		return DecodeStringArray(r)
	default:
		return nil, ErrEntryNoSuchType
	}
}

type ValueBoolean struct {
	Value    bool
	RawValue []byte
}

func DecodeBoolean(r io.Reader) (*ValueBoolean, error) {
	val := make([]byte, 1)
	_, readErr := io.ReadFull(r, val)
	if readErr != nil {
		return nil, readErr
	}
	entry := &ValueBoolean{RawValue: val}
	if entry.RawValue[0] == BoolFalse {
		entry.Value = false
		return entry, nil
	} else if entry.RawValue[0] == BoolTrue {
		entry.Value = true
		return entry, nil
	} else {
		return nil, ErrEntryDataInvalid
	}
}

func BuildBoolean(value bool) *ValueBoolean {
	var rawValue []byte
	if value {
		rawValue = []byte{BoolTrue}
	} else {
		rawValue = []byte{BoolFalse}
	}
	return &ValueBoolean{
		Value:    value,
		RawValue: rawValue,
	}
}

func (entry *ValueBoolean) UpdateRaw(r io.Reader) error {
	newEntry, entryErr := DecodeBoolean(r)
	if entryErr != nil {
		return entryErr
	}
	return mergo.MergeWithOverwrite(entry, *newEntry)
}

func (entry *ValueBoolean) GetRaw() []byte {
	return entry.RawValue
}

func (entry *ValueBoolean) UpdateValue(value bool) error {
	return mergo.MergeWithOverwrite(entry, *BuildBoolean(value))
}

type ValueString struct {
	Value    string
	RawValue []byte
}

func DecodeString(r io.Reader) (*ValueString, error) {
	uleb, ulebData, ulebErr := DecodeAndSaveULEB128(r)
	if ulebErr != nil {
		return nil, ulebErr
	}
	data := make([]byte, uleb)
	_, readErr := io.ReadFull(r, data)
	if readErr != nil {
		return nil, readErr
	}
	return &ValueString{
		Value:    string(data),
		RawValue: append(ulebData, data...),
	}, nil
}

func BuildString(value string) *ValueString {
	stringBytes := []byte(value)
	rawValue := append(EncodeULEB128(uint32(len(stringBytes))), stringBytes...)
	return &ValueString{
		Value:    value,
		RawValue: rawValue,
	}
}

func (entry *ValueString) UpdateRaw(r io.Reader) error {
	newEntry, newErr := DecodeString(r)
	if newErr != nil {
		return newErr
	}
	return mergo.MergeWithOverwrite(entry, *newEntry)
}

func (entry *ValueString) GetRaw() []byte {
	return entry.RawValue
}

func (entry *ValueString) UpdateValue(value string) error {
	return mergo.MergeWithOverwrite(entry, *BuildString(value))
}

type ValueDouble struct {
	Value    float64
	RawValue []byte
}

func DecodeDouble(r io.Reader) (*ValueDouble, error) {
	data := make([]byte, 8)
	_, readErr := io.ReadFull(r, data)
	if readErr != nil {
		return nil, readErr
	}
	return &ValueDouble{
		Value:    BytesToFloat64(data),
		RawValue: data,
	}, nil
}

func BuildDouble(value float64) *ValueDouble {
	return &ValueDouble{
		Value:    value,
		RawValue: Float64ToBytes(value),
	}
}

func (entry *ValueDouble) UpdateRaw(r io.Reader) error {
	newEntry, newErr := DecodeDouble(r)
	if newErr != nil {
		return newErr
	}
	return mergo.MergeWithOverwrite(entry, *newEntry)
}

func (entry *ValueDouble) GetRaw() []byte {
	return entry.RawValue
}

func (entry *ValueDouble) UpdateValue(value float64) error {
	return mergo.MergeWithOverwrite(entry, *BuildDouble(value))
}

type ValueRaw struct {
	Value    []byte
	RawValue []byte
}

func DecodeRaw(r io.Reader) (*ValueRaw, error) {
	uleb, ulebData, ulebErr := DecodeAndSaveULEB128(r)
	if ulebErr != nil {
		return nil, ulebErr
	}
	data := make([]byte, uleb)
	_, readErr := io.ReadFull(r, data)
	if readErr != nil {
		return nil, readErr
	}
	return &ValueRaw{
		Value:    data,
		RawValue: append(ulebData, data...),
	}, nil
}

func BuildRaw(value []byte) *ValueRaw {
	return &ValueRaw{
		Value:    value,
		RawValue: append(EncodeULEB128(uint32(len(value))), value...),
	}
}

func (entry *ValueRaw) UpdateRaw(r io.Reader) error {
	newEntry, newErr := DecodeRaw(r)
	if newErr != nil {
		return newErr
	}
	return mergo.MergeWithOverwrite(entry, *newEntry)
}

func (entry *ValueRaw) GetRaw() []byte {
	return entry.RawValue
}

func (entry *ValueRaw) UpdateValue(value []byte) error {
	return mergo.MergeWithOverwrite(entry, *BuildRaw(value))
}

type ValueBooleanArray struct {
	elements []*ValueBoolean
}

func DecodeBooleanArray(r io.Reader) (*ValueBooleanArray, error) {
	indexData := make([]byte, 1)
	_, readErr := io.ReadFull(r, indexData)
	if readErr != nil {
		return nil, readErr
	}
	index := uint8(indexData[0])
	elements := make([]*ValueBoolean, index)
	for i := uint8(0); i < index; i++ {
		boolean, decodeErr := DecodeBoolean(r)
		if decodeErr != nil {
			return nil, decodeErr
		}
		elements[i] = boolean
	}
	return &ValueBooleanArray{
		elements: elements,
	}, nil
}

func BuildBooleanArray(values []*ValueBoolean) *ValueBooleanArray {
	var index uint8
	if len(values) > 255 {
		index = 255
	} else {
		index = uint8(len(values))
	}
	return &ValueBooleanArray{
		elements: values[:index],
	}
}

func (array *ValueBooleanArray) Get(index uint8) (EntryValue, error) {
	if index >= uint8(len(array.elements)) {
		return nil, ErrArrayIndexOutOfBounds
	}
	return array.elements[index], nil
}

func (array *ValueBooleanArray) Update(index uint8, entry EntryValue) error {
	boolean, ok := entry.(*ValueBoolean)
	if !ok {
		return ErrEntryCastInvalid
	}
	if index >= uint8(len(array.elements)) {
		return ErrArrayIndexOutOfBounds
	}
	return mergo.MergeWithOverwrite(array.elements[index], *boolean)
}

func (array *ValueBooleanArray) Add(entry EntryValue) error {
	boolean, ok := entry.(*ValueBoolean)
	if !ok {
		return ErrEntryCastInvalid
	}
	if len(array.elements) > 255 {
		return ErrArrayOutOfSpace
	}
	array.elements = append(array.elements, boolean)
	return nil
}

func (array *ValueBooleanArray) GetRaw() []byte {
	data := []byte{byte(uint8(len(array.elements)))}
	var i uint8 = 0
	for ; i < uint8(len(array.elements)); i++ {
		data = append(data, array.elements[i].RawValue...)
	}
	return data
}

type ValueDoubleArray struct {
	elements []*ValueDouble
}

func DecodeDoubleArray(r io.Reader) (*ValueDoubleArray, error) {
	indexData := make([]byte, 1)
	_, readErr := io.ReadFull(r, indexData)
	if readErr != nil {
		return nil, readErr
	}
	index := uint8(indexData[0])
	elements := make([]*ValueDouble, index)
	for i := uint8(0); i < index; i++ {
		double, decodeErr := DecodeDouble(r)
		if decodeErr != nil {
			return nil, decodeErr
		}
		elements[i] = double
	}
	return &ValueDoubleArray{
		elements: elements,
	}, nil
}

func BuildDoubleArray(values []*ValueDouble) *ValueDoubleArray {
	var index uint8
	if len(values) > 255 {
		index = 255
	} else {
		index = uint8(len(values))
	}
	return &ValueDoubleArray{
		elements: values[:index],
	}
}

func (array *ValueDoubleArray) Get(index uint8) (EntryValue, error) {
	if index >= uint8(len(array.elements)) {
		return nil, ErrArrayIndexOutOfBounds
	}
	return array.elements[index], nil
}

func (array *ValueDoubleArray) Update(index uint8, entry EntryValue) error {
	double, ok := entry.(*ValueDouble)
	if !ok {
		return ErrEntryCastInvalid
	}
	if index >= uint8(len(array.elements)) {
		return ErrArrayIndexOutOfBounds
	}
	return mergo.MergeWithOverwrite(&array.elements[index], double)
}

func (array *ValueDoubleArray) Add(entry EntryValue) error {
	double, ok := entry.(*ValueDouble)
	if !ok {
		return ErrEntryCastInvalid
	}
	if len(array.elements) > 255 {
		return ErrArrayOutOfSpace
	}
	array.elements = append(array.elements, double)
	return nil
}

func (array *ValueDoubleArray) GetRaw() []byte {
	data := []byte{byte(uint8(len(array.elements)))}
	var i uint8 = 0
	for ; i < uint8(len(array.elements)); i++ {
		data = append(data, array.elements[i].RawValue...)
	}
	return data
}

type ValueStringArray struct {
	elements []*ValueString
}

func DecodeStringArray(r io.Reader) (*ValueStringArray, error) {
	indexData := make([]byte, 1)
	_, readErr := io.ReadFull(r, indexData)
	if readErr != nil {
		return nil, readErr
	}
	index := uint8(indexData[0])
	elements := make([]*ValueString, index)
	for i := uint8(0); i < index; i++ {
		string, decodeErr := DecodeString(r)
		if decodeErr != nil {
			return nil, decodeErr
		}
		elements[i] = string
	}
	return &ValueStringArray{
		elements: elements,
	}, nil
}

func BuildStringArray(values []*ValueString) *ValueStringArray {
	var index uint8
	if len(values) > 255 {
		index = 255
	} else {
		index = uint8(len(values))
	}
	return &ValueStringArray{
		elements: values[:index],
	}
}

func (array *ValueStringArray) Get(index uint8) (EntryValue, error) {
	if index >= uint8(len(array.elements)) {
		return nil, ErrArrayIndexOutOfBounds
	}
	return array.elements[index], nil
}

func (array *ValueStringArray) Update(index uint8, entry EntryValue) error {
	string, ok := entry.(*ValueString)
	if !ok {
		return ErrEntryCastInvalid
	}
	if index >= uint8(len(array.elements)) {
		return ErrArrayIndexOutOfBounds
	}
	return mergo.MergeWithOverwrite(array.elements[index], *string)
}

func (array *ValueStringArray) Add(entry EntryValue) error {
	string, ok := entry.(*ValueString)
	if !ok {
		return ErrEntryCastInvalid
	}
	if len(array.elements) > 255 {
		return ErrArrayOutOfSpace
	}
	array.elements = append(array.elements, string)
	return nil
}

func (array *ValueStringArray) GetRaw() []byte {
	data := []byte{byte(uint8(len(array.elements)))}
	var i uint8 = 0
	for ; i < uint8(len(array.elements)); i++ {
		data = append(data, array.elements[i].RawValue...)
	}
	return data
}
