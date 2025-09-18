#!/usr/bin/env python3
# -*- coding: utf-8 -*-

import json
import os
from typing import List, Dict, Any

def convert_travelplanner_data(input_file: str, output_file: str, dataset_type: str) -> None:
    """
    Convert TravelPlanner dataset to the specified format.
    
    Args:
        input_file: Path to the input JSON file
        output_file: Path to the output JSON file
        dataset_type: Type of dataset (train, eval, validation)
    """
    print(f"Processing {dataset_type} dataset: {input_file}")
    
    # Check if input file exists
    if not os.path.exists(input_file):
        print(f"Warning: Input file {input_file} does not exist. Skipping.")
        return
    
    try:
        # Load the input JSON file
        with open(input_file, 'r', encoding='utf-8') as f:
            data = json.load(f)
        
        print(f"Loaded {len(data)} records from {input_file}")
        
        # Convert each record to the target format
        converted_data = []
        
        for idx, record in enumerate(data):
            # Extract query for question field
            question = record.get('query', '')
            
            # Extract and combine fields for analysis
            days = record.get('days', 0)
            visiting_city_number = record.get('visiting_city_number', 0)
            people_number = record.get('people_number', 0)
            local_constraint = record.get('local_constraint', '')
            
            # Create analysis string by combining the extracted fields
            analysis_parts = []
            if days > 0:
                analysis_parts.append(f"Duration: {days} days")
            if visiting_city_number > 0:
                analysis_parts.append(f"Cities to visit: {visiting_city_number}")
            if people_number > 0:
                analysis_parts.append(f"Number of people: {people_number}")
            
            # Handle local_constraint - it might be a dict or string
            if local_constraint:
                if isinstance(local_constraint, dict):
                    # Extract non-None values from the constraint dict
                    constraint_parts = []
                    for key, value in local_constraint.items():
                        if value and value != "None" and str(value).lower() != "none":
                            constraint_parts.append(f"{key}: {value}")
                    if constraint_parts:
                        analysis_parts.append(f"Local constraints: {', '.join(constraint_parts)}")
                elif isinstance(local_constraint, str) and local_constraint.strip():
                    analysis_parts.append(f"Local constraints: {local_constraint}")
            
            if analysis_parts:
                analysis = "The travel planning problem requires " + ", ".join(analysis_parts) + "."
            else:
                analysis = "The travel planning problem requires planning a trip."
            
            # Create the converted record
            converted_record = {
                "id": idx,
                "question": question,
                "analysis": analysis
            }
            
            converted_data.append(converted_record)
        
        # Save the converted data
        os.makedirs(os.path.dirname(output_file), exist_ok=True)
        
        with open(output_file, 'w', encoding='utf-8') as f:
            json.dump(converted_data, f, indent=2, ensure_ascii=False)
        
        print(f"Successfully converted {len(converted_data)} records to {output_file}")
        
    except json.JSONDecodeError as e:
        print(f"Error: Failed to parse JSON file {input_file}: {e}")
    except Exception as e:
        print(f"Error: Failed to process file {input_file}: {e}")

def main():
    """Main function to convert all three TravelPlanner datasets."""
    
    # Define the base paths
    base_dataset_path = "../../../../dataset/travelplanner"
    output_base_path = "./"
    
    # Define the dataset configurations
    datasets = [
        {
            "eval": 0,
            "mode": "train",
            "input_path": f"{base_dataset_path}/train/travelplanner_train_split.json",
            "output_path": f"{output_base_path}/anal_train.json"
        },
        {
            "eval": 1,
            "mode": "eval", 
            "input_path": f"{base_dataset_path}/train/travelplanner_eval_split.json",
            "output_path": f"{output_base_path}/anal_eval.json"
        },
        {
            "eval": 2,
            "mode": "validation",
            "input_path": f"{base_dataset_path}/validation/travelplanner_validation_dataset.json",
            "output_path": f"{output_base_path}/anal_validation.json"
        }
    ]
    
    print("Starting TravelPlanner dataset conversion...")
    print("=" * 60)
    
    # Process each dataset
    for dataset_config in datasets:
        convert_travelplanner_data(
            input_file=dataset_config["input_path"],
            output_file=dataset_config["output_path"],
            dataset_type=dataset_config["mode"]
        )
        print("-" * 40)
    
    print("All datasets conversion completed!")
    
    # Print summary
    print("\nSummary:")
    for dataset_config in datasets:
        if os.path.exists(dataset_config["output_path"]):
            with open(dataset_config["output_path"], 'r', encoding='utf-8') as f:
                data = json.load(f)
                print(f"  {dataset_config['mode']}: {len(data)} records -> {dataset_config['output_path']}")
        else:
            print(f"  {dataset_config['mode']}: Failed to create output file")

if __name__ == "__main__":
    main()
