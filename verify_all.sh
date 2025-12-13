#!/bin/bash
# Get redirects to follow exam flow
COOKIE="cookies_verify.txt"
rm -f $COOKIE

# Start Exam
LOC=$(curl -s -i -c $COOKIE -X POST http://localhost:8080/exam/start -d "subject_id=1" | grep Location | awk '{print $2}' | tr -d '\r')
echo "Exam started at $LOC"

# Get total questions
# Since we don't know total, we just loop until we get redirected to result
for i in {1..70}; do
  URL="http://localhost:8080/exam/${LOC##*/exam/}/${i##*/}/question/$i"
  # We need the session ID correct.
  # The LOC is /exam/SESSIONID/question/1
  # So base URL is http://localhost:8080/exam/SESSIONID/question/$i
  
  SESSION_ID=$(echo $LOC | cut -d'/' -f3)
  URL="http://localhost:8080/exam/$SESSION_ID/question/$i"
  
  CONTENT=$(curl -s -b $COOKIE "$URL")
  
  if echo "$CONTENT" | grep -q "Review Your Answers"; then
     echo "Reached end of exam at index $i"
     break
  fi

  # Extract QID
  QID=$(echo "$CONTENT" | grep 'name="question_id"' | grep -o 'value="[0-9]*"' | cut -d'"' -f2)
  if [ -z "$QID" ]; then
    continue 
  fi
  
  # Extract Input Type (take the first one)
  INPUT_TYPE=$(echo "$CONTENT" | grep 'name="option_id"' | head -n 1 | grep -o 'type="[^"]*"' | cut -d'"' -f2)
  
  # Get DB Type
  DB_TYPE=$(sqlite3 simsexam.db "SELECT type FROM questions WHERE id=$QID")
  
  # Check
  if [ "$DB_TYPE" == "single" ] && [ "$INPUT_TYPE" != "radio" ]; then
    echo "MISMATCH at QID $QID (Index $i): DB says single, Rendered $INPUT_TYPE"
    # echo "$CONTENT"
  elif [ "$DB_TYPE" == "multiple" ] && [ "$INPUT_TYPE" != "checkbox" ]; then
    echo "MISMATCH at QID $QID (Index $i): DB says multiple, Rendered $INPUT_TYPE"
  else
    # echo "OK: QID $QID is $DB_TYPE and rendered $INPUT_TYPE"
    :
  fi
done
echo "Verification complete."
