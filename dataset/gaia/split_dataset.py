import json
import random
import os

def process_gaia_dataset():
    """
    Reads GAIA dataset JSON files, splits them into training and validation sets,
    and writes them back to new files.
    """
    # Base directory where the script and data files are located
    base_dir = os.path.dirname(os.path.abspath(__file__))

    # Input file paths
    file_paths = {
        1: os.path.join(base_dir, 'level_1_val_filtered.json'),
        2: os.path.join(base_dir, 'level_2_val_filtered.json'),
        3: os.path.join(base_dir, 'level_3_val_filtered.json'),
    }

    all_data = []
    level_counts = {}

    print("Reading data files and counting questions...")
    # Read data from each file
    for level, path in file_paths.items():
        try:
            with open(path, 'r', encoding='utf-8') as f:
                data = json.load(f)
                level_counts[level] = len(data)
                for item in data:
                    # Store data with its original level
                    all_data.append({'level': level, 'data': item})
        except FileNotFoundError:
            print(f"Error: File not found at {path}")
            return
        except json.JSONDecodeError:
            print(f"Error: Could not decode JSON from {path}")
            return

    print("\nQuestion counts per level:")
    for level, count in level_counts.items():
        print(f"  Level {level}: {count} questions")
    print(f"Total questions: {len(all_data)}")

    # Randomly shuffle the data
    random.shuffle(all_data)

    # Split data into training and validation sets
    num_train_samples = 10
    train_samples = all_data[:num_train_samples]
    val_samples = all_data[num_train_samples:]

    print(f"\nRandomly selected {num_train_samples} samples for the training set.")
    print(f"Remaining {len(val_samples)} samples will be the new validation set.")

    # Separate samples by level
    train_by_level = {1: [], 2: [], 3: []}
    val_by_level = {1: [], 2: [], 3: []}

    for sample in train_samples:
        train_by_level[sample['level']].append(sample['data'])

    for sample in val_samples:
        val_by_level[sample['level']].append(sample['data'])

    # Output file paths
    output_paths = {
        'train': {
            1: os.path.join(base_dir, 'level_1_train.json'),
            2: os.path.join(base_dir, 'level_2_train.json'),
            3: os.path.join(base_dir, 'level_3_train.json'),
        },
        'val': {
            1: os.path.join(base_dir, 'level_1_val_new.json'),
            2: os.path.join(base_dir, 'level_2_val_new.json'),
            3: os.path.join(base_dir, 'level_3_val_new.json'),
        }
    }

    print("\nWriting new training and validation files...")
    # Write training files
    for level, data in train_by_level.items():
        path = output_paths['train'][level]
        with open(path, 'w', encoding='utf-8') as f:
            json.dump(data, f, indent=4, ensure_ascii=False)
        print(f"  Created {os.path.basename(path)} with {len(data)} samples.")

    # Write validation files
    for level, data in val_by_level.items():
        path = output_paths['val'][level]
        with open(path, 'w', encoding='utf-8') as f:
            json.dump(data, f, indent=4, ensure_ascii=False)
        print(f"  Created {os.path.basename(path)} with {len(data)} samples.")
        
    print("\nProcessing complete.")

if __name__ == '__main__':
    process_gaia_dataset()
