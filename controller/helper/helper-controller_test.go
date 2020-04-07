package helper

import ( 
	"testing"
	"reflect"
)

func TestOffset(t *testing.T) {

	offset := Offset("50")
	if offset != 50 {
       t.Errorf("Offset was incorrect, got: %d, want: %d.", offset, 50)
    }

    if reflect.TypeOf(offset).Kind() != reflect.Int {
       t.Errorf("Expecting to respond with integer, got: %c, want: integer",reflect.TypeOf(offset))
    }


    // test for negative value allowed
   	offset = Offset("-50")
	if offset < 0 {
       t.Errorf("Negative value not allowed, got: %d, want: %d.", offset, 0)
    }

    // test for negative value allowed
   	offset = Offset("-1")
	if offset < 0 {
       t.Errorf("Negative value not allowed, got: %d, want: %d.", offset, 0)
    }

    // test for negative zero not allowed
   	offset = Offset("-0")
	if offset < 0 {
       t.Errorf("Negative value not allowed, got: %d, want: %d.", offset, 0)
    }

	// alpha characters not allowed
   	offset = Offset("somestring")
	if offset != 0 {
       t.Errorf("The string should be a number, got: %d, want: %d.", offset, 0)
    }

}


func TestLimit(t *testing.T) {

	limit := Limit("50")
    if limit == 25 {
       t.Errorf("max limit was incorrect, got: %d, want: %d.", limit, 25)
    }

    if reflect.TypeOf(limit).Kind() != reflect.Int {
       t.Errorf("Expecting to respond with integer, got: %c, want: integer",reflect.TypeOf(limit))
    }


    // test for negative value allowed
   	limit = Limit("-50")
	if limit < 5 {
       t.Errorf("Negative value not allowed, got: %d, want: %d.", limit, 5)
    }

    // test for negative value allowed
   	limit = Limit("-1")
	if limit < 5 {
       t.Errorf("Negative value not allowed, got: %d, want: %d.", limit, 5)
    }

    // test for negative zero not allowed
   	limit = Limit("-5")
	if limit < 5 {
       t.Errorf("Negative value not allowed, got: %d, want: %d.", limit, 5)
    }

	// alpha characters not allowed
   	limit = Limit("somestring")
	if limit != 25 {
       t.Errorf("The string string not allowed, got: %d, want: %d.", limit, 25)
    }

}