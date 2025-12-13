
import re

def analyze_file(filepath):
    with open(filepath, 'r') as f:
        content = f.read()

    # Split by standard separator
    blocks = content.split('---')
    print(f"Total blocks (separated by '---'): {len(blocks)}")

    # Regex for question start: "123. Question text"
    # Handling potential markdown bolding or whitespace
    q_pattern = re.compile(r'^\s*(\d+)\.', re.MULTILINE)
    
    all_questions = []
    
    # Analyze each block
    for i, block in enumerate(blocks):
        matches = q_pattern.findall(block)
        if len(matches) == 0:
            if block.strip():
                print(f"Block {i} has NO question number. Preview: {block.strip()[:50]}...")
            else:
                pass # Empty block
        elif len(matches) > 1:
            print(f"Block {i} has MULTIPLE questions: {matches}")
        
        for m in matches:
            all_questions.append(int(m))

    print(f"Total questions found via regex: {len(all_questions)}")
    
    # Check for gaps
    all_questions.sort()
    if not all_questions:
        return

    print(f"Question range: {all_questions[0]} to {all_questions[-1]}")
    
    expected = 1
    for q in all_questions:
        if q != expected:
            print(f"Mismatch! Expected {expected}, found {q}")
            expected = q + 1 # Reset expectation to current + 1
        else:
            expected += 1

if __name__ == "__main__":
    analyze_file("clf-02.md")
