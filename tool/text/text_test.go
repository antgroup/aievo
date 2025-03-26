package text

import "testing"

func TestSensitivityDetector_Process(t *testing.T) {
	type fields struct {
		SensitivityDetector Processor
	}
	tests := []struct {
		name    string
		fields  fields
		want    string
		wantErr bool
	}{
		{
			name: "test1",
			fields: fields{
				SensitivityDetector: NewSensitivityDetector(
					[]string{"sensitive", "words"},
					"This is a test text containing sensitive words.",
					AllSensitiveWords),
			},
			want:    "sensitive,words",
			wantErr: false,
		},
		{
			name: "test2",
			fields: fields{
				SensitivityDetector: NewSensitivityDetector(
					[]string{"sensitive", "words"},
					"This is a test text containing sensitive words.",
					FirstSensitiveWord),
			},
			want:    "sensitive",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.fields.SensitivityDetector.Process()
			if (err != nil) != tt.wantErr {
				t.Errorf("Process() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Process() got = %v, want %v", got, tt.want)
			}
		})
	}
}
