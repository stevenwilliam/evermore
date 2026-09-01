ALTER TABLE IF EXISTS staff_profile DROP CONSTRAINT IF EXISTS staff_profile_kitchen_fk;
DROP TABLE IF EXISTS out_of_range_attempt, kitchen_capacity, kitchen_operating_day,
    kitchen_slot, kitchen, scheduled_meal_item, scheduled_meal, delivery_time_slot,
    food_photo, food_diet_type, food_allergen, food_nutrition, food, allergen,
    diet_subtype, diet_type CASCADE;
