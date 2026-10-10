env CGO_ENABLED=1 go test -race ./sqlite ./driver -run \^\(TestReviewSecondary\|TestSecondaryRange\|TestDriverSecondaryRange\) -count=1 -v 
