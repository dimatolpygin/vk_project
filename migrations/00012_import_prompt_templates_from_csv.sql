-- +goose Up
-- +goose StatementBegin

-- Импортирует категории и шаблоны из предоставленных CSV в idempotent-режиме.
-- Используются только пол, категория, название кнопки и сам prompt.

WITH category_seed AS (
    SELECT *
    FROM jsonb_to_recordset($seed$[
  {
    "name": "Улица",
    "gender": "male",
    "sort_order": 1
  },
  {
    "name": "Путешествия",
    "gender": "male",
    "sort_order": 2
  },
  {
    "name": "Бизнес",
    "gender": "male",
    "sort_order": 3
  },
  {
    "name": "Кафе",
    "gender": "male",
    "sort_order": 4
  },
  {
    "name": "Студия",
    "gender": "male",
    "sort_order": 5
  },
  {
    "name": "Машина",
    "gender": "male",
    "sort_order": 6
  },
  {
    "name": "Путешествия",
    "gender": "female",
    "sort_order": 1
  },
  {
    "name": "Бизнес",
    "gender": "female",
    "sort_order": 2
  },
  {
    "name": "Кафе",
    "gender": "female",
    "sort_order": 3
  },
  {
    "name": "Селфи",
    "gender": "female",
    "sort_order": 4
  },
  {
    "name": "Студия",
    "gender": "female",
    "sort_order": 5
  },
  {
    "name": "Интерьер",
    "gender": "female",
    "sort_order": 6
  },
  {
    "name": "Новый Год",
    "gender": "female",
    "sort_order": 7
  },
  {
    "name": "Спорт",
    "gender": "female",
    "sort_order": 8
  },
  {
    "name": "Улица",
    "gender": "female",
    "sort_order": 9
  },
  {
    "name": "Праздники",
    "gender": "female",
    "sort_order": 10
  },
  {
    "name": "Машина",
    "gender": "female",
    "sort_order": 11
  }
]$seed$::jsonb)
        AS x(name TEXT, gender TEXT, sort_order INT)
),
updated_categories AS (
    UPDATE categories AS c
    SET sort_order = s.sort_order,
        is_active = TRUE
    FROM category_seed AS s
    WHERE c.name = s.name
      AND c.gender = s.gender
    RETURNING c.id
)
INSERT INTO categories (name, gender, sort_order, is_active)
SELECT s.name, s.gender, s.sort_order, TRUE
FROM category_seed AS s
WHERE NOT EXISTS (
    SELECT 1
    FROM categories AS c
    WHERE c.name = s.name
      AND c.gender = s.gender
);

WITH prompt_seed AS (
    SELECT *
    FROM jsonb_to_recordset($seed$[
  {
    "category_name": "Улица",
    "category_gender": "male",
    "name": "Улица",
    "prompt": "mid-shot portrait of a man leaning shoulder-first against a raw concrete wall, hands casually in pockets, monochrome outfit (black t-shirt, charcoal trousers), soft overcast ambient light, muted tones, shallow depth of field; shot on a 50 mm lens at f/2.2, clean modern fashion mood, minimal editorial styling",
    "gender": "male",
    "sort_order": 1
  },
  {
    "category_name": "Путешествия",
    "category_gender": "male",
    "name": "Горы",
    "prompt": "a man standing on a rock ledge overlooking deep mountain valleys, hands in coat pockets, wind lifting the hem slightly, cool ambient tones, minimalist composition, shot on an 85mm lens at f/2, cinematic solitude mood. Use the attached face images as the exact identity reference and generate the same person. Do not change the shape, features and proportions of the face from the attached photographs.",
    "gender": "male",
    "sort_order": 1
  },
  {
    "category_name": "Бизнес",
    "category_gender": "male",
    "name": "Бизнес портрет",
    "prompt": "Keep the facial features of the person in the uploaded image exactly consistent . Dress them in a professional navy blue business suit with a white shirt. Background : Place the subject against a clean, solid dark gray studio photography backdrop . The background should have a subtle gradient , slightly lighter behind the subject and darker towards the edges (vignette effect). There should be no other objects. Photography Style : Shot on a Sony A7III with an 85mm f/1.4 lens , creating a flattering portrait compression. Lighting : Use a classic three-point lighting setup . The main key light should create soft, defining shadows on the face. A subtle rim light should separate the subject's shoulders and hair from the dark background. Crucial Details : Render natural skin texture with visible pores , not an airbrushed look. Add natural catchlights to the eyes . The fabric of the suit should show a subtle wool texture.Final image should be an ultra-realistic, 8k professional headshot. Use the attached face images as the exact identity reference and generate the same person. Do not change the shape, features and proportions of the face from the attached photographs.",
    "gender": "male",
    "sort_order": 1
  },
  {
    "category_name": "Кафе",
    "category_gender": "male",
    "name": "Кафе",
    "prompt": "Keep the facial features of the person in the uploaded image exactly consistent . Style : A cinematic, emotional portrait shot on Kodak Portra 400 film . Setting : An urban street coffee shop window at Golden Hour (sunset) . Warm, nostalgic lighting hitting the side of the face. Atmosphere : Apply a subtle film grain and soft focus to create a dreamy, storytelling vibe. Action : The subject is looking slightly away from the camera, holding a coffee cup, with a relaxed, candid expression. Details : High quality, depth of field, bokeh background of city lights.",
    "gender": "male",
    "sort_order": 1
  },
  {
    "category_name": "Студия",
    "category_gender": "male",
    "name": "Casual",
    "prompt": "man sitting on a white contemporary sofa with relaxed posture, natural daylight from the side, monochrome beige- white outfit, 35mm lens at f/2.5, clean Scandinavian mood",
    "gender": "male",
    "sort_order": 1
  },
  {
    "category_name": "Студия",
    "category_gender": "male",
    "name": "Casual 2",
    "prompt": "man standing by a tall window with hands in pockets, soft ambient light falling across his face, clean modern apartment, neutral-toned outfit, 50mm lens at f/2, quiet masculine atmosphere. Use the attached face images as the exact identity reference and generate the same person. Do not change the shape, features and proportions of the face from the attached photographs.",
    "gender": "male",
    "sort_order": 2
  },
  {
    "category_name": "Машина",
    "category_gender": "male",
    "name": "Машина",
    "prompt": "cinematic close-up of a man seen through a slightly fogged car window, soft interior light shaping his face, reflections of city lights on the glass, calm focused expression, 85mm lens at f/1.8, luxury noir mood",
    "gender": "male",
    "sort_order": 1
  },
  {
    "category_name": "Машина",
    "category_gender": "male",
    "name": "Машина 2",
    "prompt": "a man sitting inside a premium car, interior lighting shaping his face, clean reflections on glass, hands on steering wheel, calm confident expression, cinematic luxury aesthetic (no posing next to the car)",
    "gender": "male",
    "sort_order": 2
  },
  {
    "category_name": "Студия",
    "category_gender": "male",
    "name": "Студия",
    "prompt": "mid-shot of a man with sharp rectangular shadow patterns over his hands and forearms, black tailored outfit, subtle pose with hands partially raised, matte backdrop, shot on a 50mm lens at f/2, sleek fashion composition. Use the attached face images as the exact identity reference and generate the same person. Do not change the shape, features and proportions of the face from the attached photographs.",
    "gender": "male",
    "sort_order": 3
  },
  {
    "category_name": "Улица",
    "category_gender": "male",
    "name": "Улица 2",
    "prompt": "seated portrait of a man sitting on the ground near a concrete wall, elbows resting on knees, relaxed forward lean, black minimal jacket, soft diffused overcast light, muted palette; shot on a 35 mm lens at f/2.5, editorial urban aesthetic with intimate depth. Use the attached face images as the exact identity reference and generate the same person. Do not change the shape, features and proportions of the face from the attached photographs.",
    "gender": "male",
    "sort_order": 2
  },
  {
    "category_name": "Улица",
    "category_gender": "male",
    "name": "Улица 3",
    "prompt": "full-body shot of a man walking slowly along a concrete wall, relaxed stride, black minimalist coat flowing slightly with movement, matte textures, overcast sky giving diffused soft light; shot on a 35 mm lens at f/2.8, straight-on angle, chic urban fashion energy",
    "gender": "male",
    "sort_order": 3
  },
  {
    "category_name": "Студия",
    "category_gender": "male",
    "name": "Студия 2",
    "prompt": "half-body studio shot of a man in a tailored black suit with a slim satin lapel, white shirt slightly unbuttoned, subtle motion in the jacket as he turns; strong hard-light setup with a clean edge rim; shot on an 85 mm lens at f/2 for creamy depth, straight-on camera angle; modern YSL editorial tension and sculptural masculine lines. Use the attached face images as the exact identity reference and generate the same person. Do not change the shape, features and proportions of the face from the attached photographs.",
    "gender": "male",
    "sort_order": 4
  },
  {
    "category_name": "Студия",
    "category_gender": "male",
    "name": "Студия 3",
    "prompt": "dramatic studio shot of a man seated on a simple black chair, back straight, elbows resting lightly on knees, directional top light creating a sculpted silhouette, minimal black outfit, controlled masculine tension, high-fashion composition",
    "gender": "male",
    "sort_order": 5
  },
  {
    "category_name": "Студия",
    "category_gender": "male",
    "name": "Портрет",
    "prompt": "close-up portrait of a man near a window wearing a soft knit sweater, warm window glow shaping his features, deep background falloff, 85mm lens at f/1.4, intimate cinematic editorial portrait. Use the attached face images as the exact identity reference and generate the same person. Do not change the shape, features and proportions of the face from the attached photographs.",
    "gender": "male",
    "sort_order": 6
  },
  {
    "category_name": "Студия",
    "category_gender": "male",
    "name": "Портрет 2",
    "prompt": "tight close-up of a man’s face with a sharp split-light shadow across the features, intense confident gaze toward the camera, matte black background, clean minimal styling, refined YSL-inspired elegance, hyperrealistic texture",
    "gender": "male",
    "sort_order": 7
  },
  {
    "category_name": "Студия",
    "category_gender": "male",
    "name": "Портрет 3",
    "prompt": "dramatic studio portrait of a man with strong directional lighting and deep shadows, black minimal outfit, sharp silhouette, confident gaze, modern YSL-inspired mood, high- fashion composition. Use the attached face images as the exact identity reference and generate the same person. Do not change the shape, features and proportions of the face from the attached photographs.",
    "gender": "male",
    "sort_order": 8
  },
  {
    "category_name": "Студия",
    "category_gender": "male",
    "name": "Портрет 4",
    "prompt": "Ultra-realistic cinematic portrait of a woman/man, leaning forward with hand touching lips, intense gaze at camera. Three-quarter angle close-up, split theatrical lighting with red gel on right casting warm glows and shadows on hair and cheek, blue neon gel on left tinting skin and eye in cool hues for dichromatic effect. Red-orange gradient background with neon rim light outlining silhouette, high-contrast vibrant color grade highlighting skin tones and hair volume. 8K details on hair strands, lip contours, eye reflections, and light gradients",
    "gender": "male",
    "sort_order": 9
  },
  {
    "category_name": "Студия",
    "category_gender": "male",
    "name": "Портрет 5",
    "prompt": "A sharply focused close-up reveals an adult Caucasian man with an average build, dressed in a sleek black turtleneck. Dramatic studio lighting sculpts his face, highlighting every pore, subtle freckle, and natural imperfection against a muted backdrop. Soft shadows and gentle reflections emphasize the glossy sheen of his skin, evoking the refined polish of a high-fashion editorial. The scene is captured through a macro lens on a professional DSLR, where tactile textures of his complexion unfold in immaculate detail. This cinematic composition balances clarity and depth, presenting a lifelike yet artfully controlled portrait that breathes authenticity and sophistication. Use the attached face images as the exact identity reference and generate the same person. Do not change the shape, features and proportions of the face from the attached photographs.",
    "gender": "male",
    "sort_order": 10
  },
  {
    "category_name": "Путешествия",
    "category_gender": "female",
    "name": "Дюны",
    "prompt": "SHOT:  • Composition: Eye-level medium shot centered on seated subject, slight right profile  • LENS EFFECTS:  • Optics: Standard lens, slight natural vignetting  • Artifacts: None  • Depth of Field: Moderate, background slightly blurred  SUBJECT:  • Description: Young adult Caucasian female, average body type, relaxed posture looking to right  • Wardrobe: Light grey oversized hoodie with subtle text, slim black pants, black classic high-top sneakers with white laces  • Grooming: Her hair, softly textured and naturally styled, frames a relaxed, genuine expression., minimal natural makeup, simple gold hoop earrings  SCENE:  • Location: Sandy beach area near modern wooden cabine.Composition is casually framed with a slight tilt, evoking the genuine intimacy and immediacy characteristic of authentic iPhone photography. Use the attached face images as the exact identity reference and generate the same person. Do not change the shape, features and proportions of the face from the attached photographs.",
    "gender": "female",
    "sort_order": 1
  },
  {
    "category_name": "Путешествия",
    "category_gender": "female",
    "name": "Горы",
    "prompt": "ultra-realistic editorial scene on a snowy mountain terrace.  \na woman lounges casually in a beige knit après-ski set — fitted sweater and leggings, paired with matching beige ugg boots and oversized sunglasses.  \nshe reclines on a deck chair with white fur draped over the armrest.  \nlighting — soft afternoon alpine sunlight, warm tones reflecting on the snow.  \nmakeup — radiant skin, bold lips, minimal eye makeup.  \ndetails — luxury chalet in the background, subtle snow sparkles, branded ski decor.  \nmood — chic, effortless après-ski glamour.  \nstyle — retro-inspired 70s ski resort editorial, clean composition.  \n8K ultra-detailed realism, snow reflections, knit texture clarity, crisp shadows.  \nUse the attached face images as the exact identity reference and generate the same person. Do not change the shape, features and proportions of the face from the attached photographs.",
    "gender": "female",
    "sort_order": 2
  },
  {
    "category_name": "Бизнес",
    "category_gender": "female",
    "name": "Бизнес",
    "prompt": "Keep the facial features of the person in the uploaded image exactly consistent . Dress them in a professional navy blue business suit with a white shirt, similar to the reference image. Background : Place the subject against a clean, solid dark gray studio photography backdrop . The background should have a subtle gradient , slightly lighter behind the subject and darker towards the edges (vignette effect). There should be no other objects. Photography Style : Shot on a Sony A7III with an 85mm f/1.4 lens , creating a flattering portrait compression. Lighting : Use a classic three-point lighting setup . The main key light should create soft, defining shadows on the face. A subtle rim light should separate the subject's shoulders and hair from the dark background. Crucial Details : Render natural skin texture with visible pores , not an airbrushed look. Add natural catchlights to the eyes . The fabric of the suit should show a subtle wool texture.Final image should be an ultra-realistic, 8k professional headshot. Use the attached face images as the exact identity reference and generate the same person. Do not change the shape, features and proportions of the face from the attached photographs.",
    "gender": "female",
    "sort_order": 1
  },
  {
    "category_name": "Кафе",
    "category_gender": "female",
    "name": "Кафе",
    "prompt": "Keep the facial features of the person in the uploaded image exactly consistent . Style : A cinematic, emotional portrait shot on Kodak Portra 400 film . Setting : An urban street coffee shop window at Golden Hour (sunset) . Warm, nostalgic lighting hitting the side of the face. Atmosphere : Apply a subtle film grain and soft focus to create a dreamy, storytelling vibe. Action : The subject is looking slightly away from the camera, holding a coffee cup, with a relaxed, candid expression. Details : High quality, depth of field, bokeh background of city lights. Use the attached face images as the exact identity reference and generate the same person. Do not change the shape, features and proportions of the face from the attached photographs.",
    "gender": "female",
    "sort_order": 1
  },
  {
    "category_name": "Путешествия",
    "category_gender": "female",
    "name": "Париж",
    "prompt": "Use the attached face images as the exact identity reference and generate the same person. Picture this woman on the balcony of a stylish expensive hotel in Paris overlooking the Eiffel Tower on a spring night. \n\n She stands with her back to the city and looks straight into the lens, leaning on the balcony railing with both hands, in a black dress with bare shoulders, nude makeup, stylish silver hoop earrings in her ears, and a silver cartie nail bracelet on her hand.  The face is detailed, the pores of the skin are visible.  Do not change the shape, features and proportions of the face from the attached photographs.  Realistic photo on iPhone 16 with flash. Use the attached face images as the exact identity reference and generate the same person. Do not change the shape, features and proportions of the face from the attached photographs.",
    "gender": "female",
    "sort_order": 3
  },
  {
    "category_name": "Селфи",
    "category_gender": "female",
    "name": "Лифтолук",
    "prompt": "Using the attached photo, take a photo taken in the elevator - a mirror selfie. The girl is wearing a dark brown or gray-brown quilted down jacket shortened at the waist with a voluminous fluffy fur collar formed from three large “pillows” of fur on each side.  Under the jacket is a dark brown tight jumpsuit.  Nude makeup, tanned skin, gloss on the lips, emphasis on the cheekbone area.  On the ears are small shining cluster earrings made of transparent crystals/rhinestones.  In his right hand he holds a black iPhone 16 in a gray case;   You can see a dark handbag in your hand.  Don't change your facial features.  Similarity 100%.  High quality mobile photography.  The girl's gaze is confident, a little playful, looking forward. Use the attached face images as the exact identity reference and generate the same person. Do not change the shape, features and proportions of the face from the attached photographs.",
    "gender": "female",
    "sort_order": 1
  },
  {
    "category_name": "Путешествия",
    "category_gender": "female",
    "name": "Хогвартс",
    "prompt": "Using the attached photo, create a hyper-realistic scene on platform 9 and 3/4, where a woman (keep her face exactly like the attached photo) is standing on the platform, waiting for the Hogwarts train.  She has a suitcase in her hands, she is wearing a Hogwarts student's robe and a Gryffindor scarf, and there is a slight smile on her face.  Around her stand other students with suitcases and bags, hurrying to their train: In the background is the famous wall through which passengers pass, and the Hogwarts train has already arrived, smoking and illuminating the station with bright headlights.  The lighting is warm, evening, creating a magical atmosphere.  The scene looks like a real photograph with cinematic 8K processing, where every detail,\n from the fabric of her robe to the smoking train, is clearly drawn, and the atmosphere\n filled with the magic and excitement of the upcoming journey.",
    "gender": "female",
    "sort_order": 4
  },
  {
    "category_name": "Студия",
    "category_gender": "female",
    "name": "Классика",
    "prompt": "create an image without changing the facial features of the girl from the attached photo. A photorealistic half-body portrait of a young woman in her mid-30s with natural warm skin tone, and calm confidence. She wears a black strapless dress. Her expression is\npoised and relaxed, eyes slightly turned toward the viewer. Lighting is diffused and\nneutral-warm, softly illuminating her cheekbones and collarbones with balanced\nshadows. The background is minimal gray with shallow depth, complementing her\noutfit tone. Captured at a three-quarter angle using a 50mm lens at f/2.8, maintaining\nrealistic proportions and cinematic clarity. Editorial-grade finish with preserved skin\ngrain, lifelike tone mapping, and premium natural polish. Use the attached face images as the exact identity reference and generate the same person. Do not change the shape, features and proportions of the face from the attached photographs.",
    "gender": "female",
    "sort_order": 1
  },
  {
    "category_name": "Интерьер",
    "category_gender": "female",
    "name": "Кино",
    "prompt": "A gritty, bohemian fashion photo, shot with a direct flash like at an underground party. A lone girl in a red evening dress is sitting in an empty, classic cinema with dark red velvet seats, her feet casually propped up on the seat in front of her. A few pieces of popcorn are scattered on the floor. The direct flash creates harsh shadows, overexposes her skin slightly, and makes the sequins on her dress sparkle violently against the moody, dark red surroundings. The atmosphere is melancholic, rebellious, and glamorously lonely. Shot on grainy 35mm film, high contrast, no smile, a contemplative and defiant look. Keep the person's facial features exactly the same as in the Image. Also, save the hairstyle and the color of the person's hair. Don't use letters or logos. Use the attached face images as the exact identity reference and generate the same person. Do not change the shape, features and proportions of the face from the attached photographs.",
    "gender": "female",
    "sort_order": 1
  },
  {
    "category_name": "Новый Год",
    "category_gender": "female",
    "name": "Маска",
    "prompt": "Cinematic holiday portrait of a woman holding a vintage bunny mask in front of her face.\n Soft dreamy motion blur, festive warm lighting, subtle bokeh, subtle glow.  Woman dressed in red party dress with sparkles or sequins, elegant style.\n Composition: close-up portrait, mask covers part of the face, one expressive eye is visible.  Softness, like in a retro film, warm colors, minimal background.  Keep the woman's facial features, proportions and hairstyle consistent with the attached reference photo.",
    "gender": "female",
    "sort_order": 1
  },
  {
    "category_name": "Новый Год",
    "category_gender": "female",
    "name": "Зеркало",
    "prompt": "Use the attached face images as the exact identity reference and generate the same person. Do not change the shape, features and proportions of the face from the attached photographs.Photo in the mirror\n A girl stands in front of a mirror, holding a phone in her hands, taking a photo in the mirror.  She is wearing a stylish red jacket and skirt that perfectly highlight her figure.  In the mirror you can see how she confidently poses, slightly raising one leg and crossing it.  On the top of the mirror, “Happy New Year” is written in large letters in red lipstick, creating a festive atmosphere.   In the distance behind it you can see a New Year tree, decorated with bright lights, golden balls and festive garlands, which adds to the magical mood.  The warm light coming from the window softly illuminates her face, giving the photo a sophisticated atmosphere. Use the attached face images as the exact identity reference and generate the same person. Do not change the shape, features and proportions of the face from the attached photographs.",
    "gender": "female",
    "sort_order": 2
  },
  {
    "category_name": "Новый Год",
    "category_gender": "female",
    "name": "С бокалом",
    "prompt": "Without changing the appearance of the woman in the photo, the viewer is presented with a festive scene filled with the atmosphere of a gentle winter celebration.  The woman is frozen in a dynamic, almost magical gesture: she looks straight into the camera, slightly parting her lips, as if blowing a kiss, while her right hand is raised to her face in a slight movement, from which sparkling confetti in the shape of stars and round shiny pieces seem to dissolve into the air.  In her left hand she carefully holds a thin, elegant glass with a golden drink that reflects the soft light.  Her hair flows freely, softly framing her face, creating a natural contrast with the dark, velvety texture of the dress, with dramatic fluffy elements on the shoulders, adding drama and volume to the look.  The light falling from the front emphasizes the shine and translucency of the outline of the glass, and the golden-yellow lights of the Christmas tree, located against a blurred background, seem to be woven from many small highlights, giving the scene a warm glow and depth.  It creates a feeling of late evening, filled with a gentle flicker and a mysterious soft light that dances on the woman’s face, enlivening her calm, slightly playful expression.  All the details of the scene are united and shrouded in a light haze, characteristic of a festive mood, creating a feeling of comfort, joy and magic of the moment when the mirror of reality reflects a sincere, slightly mysterious smile.  Shadows are carefully rendered, enhancing the volume of the image without creating sharp contrasts, maintaining the overall harmony of the composition, like a frame from a cinematic film with a warm glow and an interior filled with light and celebration, photorealistic image, high textural detail, high quality, 4K resolution. Use the attached face images as the exact identity reference and generate the same person. Do not change the shape, features and proportions of the face from the attached photographs.",
    "gender": "female",
    "sort_order": 3
  },
  {
    "category_name": "Спорт",
    "category_gender": "female",
    "name": "Канат",
    "prompt": "Create a portrait without changing facial features.  A medium full shot from a low angle shot depicts a strong, sweaty woman in workout clothes (a gym top and short shorts) performing an intense battle ropes exercise in a darkened gym or studio.\n Lighting: Dramatic, studio lighting with strong contrast (chiaroscuro).  A bright rim light illuminates the woman from behind and from the side, emphasizing her figure and the sheen of sweat on her skin.  The background is very dark, almost black, which creates the effect of the figure “emerging” from the darkness.  In the background there is fog or smoke, enhanced by a ray of light, creating an atmosphere of mysticism and tension.\n Action and Posture: The woman is in a deep, athletic stance (partial squat).  She holds the thick, heavy ropes tightly with both hands, actively raising and lowering them, creating powerful, dynamic waves in the air.  Her arms and shoulders are tense.\n Emotions and Facial Expression: Her facial expression is focused, serious and determined.  She looks ahead with a heavy gaze, completely focused on her workout, conveying a sense of effort, power and endurance. Use the attached face images as the exact identity reference and generate the same person. Do not change the shape, features and proportions of the face from the attached photographs.",
    "gender": "female",
    "sort_order": 1
  },
  {
    "category_name": "Кафе",
    "category_gender": "female",
    "name": "Патрики",
    "prompt": "A bright, airy, and joyful fashion editorial photo for a lifestyle magazine. A stylish model is sitting at a table on the glass-enclosed veranda of a cozy cafe on a sunny spring morning. She is holding a cup of cappuccino, smiling softly, enjoying a quiet breakfast. The view overlooks the picturesque, tree-lined streets and elegant early 20th-century architecture of the Patriarch's Ponds (Patriarshie Prudy) district in Moscow. Young green leaves on the trees, the first spring flowers in boxes, soft morning light creating long shadows. The atmosphere is serene, sophisticated, and full of spring freshness. Shot on a professional digital camera with a shallow depth of field, natural lighting, crisp and clean composition. Style is contemporary, chic, and photorealistic. Keep the person's facial features exactly the same as in the Image. Also, save the hairstyle and the color of the person's hair. Don't use letters or logos.",
    "gender": "female",
    "sort_order": 2
  },
  {
    "category_name": "Улица",
    "category_gender": "female",
    "name": "Зима Москва",
    "prompt": "A hyperrealistic glossy fashion editorial photoshoot for Vogue magazine. A stunning model in a luxurious, oversized winter coat or fur coat walks confidently down Tverskaya Street in Moscow on a snowy winter evening. The street is covered in pristine white snow, with large, soft snowflakes falling gently. The historic and modern buildings along the street are illuminated by the warm, golden glow of ornate street lamps, Christmas decorations, and glowing storefront windows. The scene is vibrant yet elegant, capturing the magical atmosphere of a winter night in the city center. The model's makeup is flawless, with a subtle glow, and her hair is styled elegantly, perhaps with a few snowflakes caught in it. Shot with a professional camera (like a Canon R5), using a shallow depth of field to make the model pop against the softly blurred, sparkling bokeh of city lights. The style is cinematic, luxurious, and sharply detailed, with high contrast between the warm lights and the cool blue twilight sky. Masterpiece, editorial photography. Keep the person's facial features exactly the same as in the Image.Use the attached face images as the exact identity reference and generate the same person. Do not change the shape, features and proportions of the face from the attached photographs.",
    "gender": "female",
    "sort_order": 1
  },
  {
    "category_name": "Путешествия",
    "category_gender": "female",
    "name": "В горах",
    "prompt": "Use the attached face images as the exact identity reference and generate the same person. Do not change the shape, features and proportions of the face from the attached photographs. Photorealistic alpine fashion portrait. A woman stands on a snowy slope with dramatic mountains behind her, framed from mid-thigh to slightly above her head. She faces the camera, body slightly turned, with her hands tucked into the pockets of a voluminous cream-white puffer jacket.\nShe has fair skin and soft pink lips, mostly hidden behind oversized black rectangular sunglasses. Under the jacket she wears a beige ribbed knit hood that wraps tightly around her head and neck like a balaclava, with a few strands of hair peeking out. The puffer jacket has a slightly shiny surface, large stitched segments, and fluffy shearling trim at the hem. She pairs it with straight white ski pants.\nLocation: bright Alpine midday environment with crisp sun, blue sky, and high-contrast snow-covered peaks in the distance. Color grading: bold, vivid blues and whites, strong contrast similar to the CASA-style poster, with her cream-beige tones standing out against the intense sky.\nLighting: direct sunlight from upper left adds sharp, graphic shadows under her chin and along the folds of the jacket, but snow reflection keeps overall exposure bright and clean. Camera: 35mm lens at f/4, slightly low angle to emphasize the jacket’s volume and the mountains’ scale. Composition: she stands slightly off-center, a colored ski pole or lift structure visible blurred behind her, adding a sporty touch.",
    "gender": "female",
    "sort_order": 5
  },
  {
    "category_name": "Праздники",
    "category_gender": "female",
    "name": "День рождения",
    "prompt": "Use the attached face images as the exact identity reference and generate the same person. Do not change the shape, features and proportions of the face from the attached photographs. Picture a woman sitting on a bed surrounded by white and gold balloons with the letter “HAPPY BIRTHDAY” in the background. \n Face sideways to the camera, eyes closed. She holds two golden balls in her hands, raised up, creating a festive atmosphere.  Her long hair is beautifully let down with soft waves.  She is wearing a white two-piece set with sequins and decorative sparkles that sparkle in the flash, reflecting the light.  Next to her is a large bouquet of white roses in a decorative box, and on the bed is an elegant white cake with cream trim, decorated with pearls.  Lots of gel balloons with long ribbons create an airy and festive effect.  The flash illuminates the girl's face and the dress, focusing on the golden balls and the light that reflects off the sequins on the dress and jewelry",
    "gender": "female",
    "sort_order": 1
  },
  {
    "category_name": "Бизнес",
    "category_gender": "female",
    "name": "Деловой лук",
    "prompt": "Use the attached face images as the exact identity reference and generate the same person. Do not change the shape, features and proportions of the face from the attached photographs. A dynamic studio portrait of a woman dressed in business style, holding a laptop and a black folder with documents, looking to the side. Medium shot, eye level perspective, dynamic pose. Glossy nude lips. Hair pulled back into a sleek bun, medium hoop earrings. She is wearing an unbuttoned dark gray tweed suit blazer over a crisp white button-down shirt, an elegant black tie, and a short gray pinstriped skirt. She wears cat-eye glasses with black frames. In her left hand, she holds a silver Apple MacBook, and in her right hand, a black folder with documents. Professional indoor photography studio setting against a simple, seamless, pure white background. Bright, even studio lighting in a high key. Shot with a 50mm lens, the medium depth of field with sharp focus on the woman and props is photorealistic.",
    "gender": "female",
    "sort_order": 2
  },
  {
    "category_name": "Кафе",
    "category_gender": "female",
    "name": "Ужин с мужчиной",
    "prompt": "Use the attached face images as the exact identity reference and generate the same person. Do not change the shape, features and proportions of the face from the attached photographs. Without changing the appearance of the woman in the photo, in the warm and intimate light of the evening lighting, she turned her face to her interlocutor, allowing the soft rays to capture every detail of her skin and expressive eyes that glow with tenderness and attention.  Her hair falls indistinguishably and loosely to the back of her head, creating a cozy contrast with the thin shimmering material of the golden dress that hugs her shoulders and neckline, catching the light and reflecting it with a soft glow.  A graceful necklace with sparkling stones shines on her neck, and earrings carefully highlight her ear, casting muted highlights.  Her hand, slightly touching the chin, adds thoughtfulness and grace to the pose, and slightly raised eyebrows and slightly smiling lips create an atmosphere of deep internal dialogue and emotional intimacy.  The background is indistinguishably and blurrily immersed in the play of shadows and highlights emanating from the light source behind, which gives the scene cinematic depth and intimacy of the moment.  In the foreground, the silhouette of the interlocutor is defocused, partially cropped by the frame, creating the effect of presence and concentration on the woman.  A glass glass of water and a cup of coffee on the table add everyday realism to the scene, and the overall color tone maintains a warm golden-brown palette, rich in soft transitions and volumetric shadows, enhancing the feeling of comfort and emotional richness of the conversation, photorealistic image, high texture detail, high quality, 4K resolution.",
    "gender": "female",
    "sort_order": 3
  },
  {
    "category_name": "Улица",
    "category_gender": "female",
    "name": "Сити",
    "prompt": "Use the attached face images as the exact identity reference and generate the same person. Do not change the shape, features and proportions of the face from the attached photographs. A woman is standing on a rooftop or high terrace at night. Behind her, a large Moscow City skyline is visible; tall buildings are lit up, with windows glowing like tiny dots. The sky is overcast and cloudy, but the city lights softly illuminate both the environment and the woman’s face.\nShe is standing sideways, looking directly at the camera. Her hair is soft, wavy, slightly blown by the wind, with only a few fine strands naturally resting on her face, while the rest of her hair stays naturally around her head.\nShe has a subtle, sweet, and confident smile. Her eyes are half-open, looking directly at the camera, giving the photo a warm, engaging energy. The corners of her lips show a faint smile.\nShe is wearing a thick dark grey hooded hoodie. The hoodie is loose and comfortable. On the back, there is a large pattern or lettering, which stands out in white tones against the black-and-white fabric. The hood sits behind her, giving volume around her shoulders.\nThe buildings in the background are modern and tall; the lights add a sense of nightlife, vibrancy, and movement. On the left side of the photo is a more rounded skyscraper, while on the right there are more angular, layered structures. The building lights appear slightly blurred.\nHer posture is slightly turned to the side, with her shoulder closer to the camera.\nUse the face from the photo I provided.",
    "gender": "female",
    "sort_order": 2
  },
  {
    "category_name": "Новый Год",
    "category_gender": "female",
    "name": "Карусель",
    "prompt": "Use the attached face images as the exact identity reference and generate the same person. Do not change the shape, features and proportions of the face from the attached photographs. A photo of a woman standing on a moving carousel platform. She is holding the carousel pole and looking at the camera. The shot is from a medium shot at eye level. She is wearing a monochromatic, all-white winter outfit consisting of leggings, a turtleneck, a cropped fluffy faux fur jacket, and a matching fluffy faux fur hat. She wears white socks and fuzzy slip-on boots with red accents. In her left hand, she holds a small beige and white leather handbag with gold hardware. The setting is a Moscow city square during a Christmas market at dusk. She is standing on an antique carousel and behind her is a large, ornate historic building, the GUM department store in Moscow. The building is brightly lit with thousands of warm, golden fairy lights outlining the architecture, creating a festive atmosphere. The lighting is mostly bright, with warm backlighting and a halo from the fairy lights and the background lighting of the market contrasting with the blue twilight sky. The shadows are soft, shot with an 85mm portrait lens, shallow depth of field, creamy bokeh from distant light sources, photorealistic.",
    "gender": "female",
    "sort_order": 4
  },
  {
    "category_name": "Новый Год",
    "category_gender": "female",
    "name": "Подарки",
    "prompt": "Use the attached face images as the exact identity reference and generate the same person. Do not change the shape, features and proportions of the face from the attached photographs. A woman squats sideways on a tall stack of red gift boxes, resting her elbow on her knees. Her chin rests on the back of her hand, looking directly at the camera. A three-quarter shot from the top of her head to her feet. The composition is centered with the woman on the right and a column of boxes on the left and below her. She has an even matte skin tone, neutral beige-brown matte eyeshadow on her eyelids, dark, long, natural eyelashes, and a warm, natural nude lipstick. Her hair is pulled back into a high bun on top of her head, with individual strands escaping around her face. She wears a long, voluminous white coat made of textured material with long, fluffy pile that completely covers her arms and shoulders. She wears thick, glossy red tights under the coat, and red ankle boots. Numerous red gift boxes of various sizes are arranged around the woman, some neatly stacked to the left, reaching shoulder level, others lying underneath her, and the woman sitting on them. All the boxes are matte red with smooth edges, some tied with red satin ribbons. The setting is a simple interior location with a plain white wall without decoration. Bright, directional lighting. Highly detailed, photorealistic, shot on a 50mm or 85mm lens.",
    "gender": "female",
    "sort_order": 5
  },
  {
    "category_name": "Кафе",
    "category_gender": "female",
    "name": "За столиком кафе",
    "prompt": "Use the attached face images as the exact identity reference and generate the same person. Do not change the shape, features and proportions of the face from the attached photographs. A photo of a woman sitting at a cafe table, she rests her cheek on her left hand and looks slightly up and to the side with a thoughtful expression, medium shot from the waist, centered composition with the woman in the center and the cafe interior filling the background. Skin with an even matte tone, eyelids with warm brown eyeshadow, eyelashes long and accentuated with mascara, lips with peachy nude lipstick. She wears a loose white V-neck blouse with frill trim, a camel-colored knit cardigan draped over her shoulders, the sleeves are long and slightly loose, a thin gold bracelet and a white pearl bracelet are on her left wrist, and a simple gold ring is on her right hand. In front of her on a marble cafe table are several white ceramic mugs with green stripes, one with a metal spoon inside, a small stainless steel creamer, metal cutlery wrapped in a napkin, she is sitting on a chair with caramel brown upholstery, in the background is a bar counter with bottles, cups and a coffee machine, behind her is a wall of green open shelves filled with glassware, jars, bottles, decorative objects and framed prints, a large black cafe menu board hangs in the center, the text is illegible due to the bokeh effect, another person is sitting at the counter with his back to the camera, overhead is a round pendant light made of ribbed glass with a visible warm color. The bulb is positioned above the woman's head, the lighting is warm, soft room light, a combination of ambient lighting and diffused lighting typical of a cafe. Photorealistic image, shot with an 85mm lens, strong blur of the shelves and the patron-figure in the background.",
    "gender": "female",
    "sort_order": 4
  },
  {
    "category_name": "Новый Год",
    "category_gender": "female",
    "name": "Каток",
    "prompt": "Use the attached face images as the exact identity reference and generate the same person. Do not change the shape, features and proportions of the face from the attached photographs. A woman stands on an ice rink with her arms outstretched. The composition is dynamic, with her body facing the camera, her head slightly turned to the right, her gaze directed past the lens. People are skating around her on the ice. The woman wears a medium shot, mid-thigh length, casual makeup, an even matte skin tone, and thick, long eyelashes. She is wearing a voluminous, oversized, mid-thigh-length beige down jacket with horizontal quilting, wide sleeves, and a high collar. A plain beige knit turtleneck is visible underneath the jacket. She wears a thick, knitted beanie hat that matches the jacket without any embellishments. A thin chain strap from a small, rectangular, brown leather bag with a flap and a metal buckle at the waist is slung over her shoulder. Loose, wide-legged beige trousers are visible underneath. Setting: Russia, an outdoor city skating rink against the backdrop of a historic building with bright architectural lighting and illumination, on the left in the frame is a tall Christmas tree decorated with glowing garlands and large toys, multi-colored garlands are strung above the rink, in the background are many people in winter clothes skating and standing at the fences, the sky is dark at night with no visible stars, the lighting is bright artificial, bright festive night lighting, several sources: powerful rink spotlights create general uniform illumination of the stage, additional warm light from the facade of the building and garlands, highly detailed, photorealistic, shot on a 35mm lens.",
    "gender": "female",
    "sort_order": 6
  },
  {
    "category_name": "Путешествия",
    "category_gender": "female",
    "name": "Девушка у пруда",
    "prompt": "Use the attached face images as the exact identity reference and generate the same person. Do not change the shape, features and proportions of the face from the attached photographs. Full-length, fashion portrait of a  woman from an uploaded photograph, her face unchanged, standing in the middle of a pond or densely covered with giant floating leaves of Victoria water lily (Victoria Amazonica). The woman stands in the center of the frame, looking down, posing with a dramatic or pensive pose.\n\n Clothes: The woman is wearing a minimalist beach set, crocheted knitwear or large mesh, black.  The set consists of a long sleeved top and a long skirt with a high slit that ties at the hip, showing off her legs and figure.  Hair pulled back with a wet effect, one strand comes out \n Tropical, lush, humid landscape.  The background consists of green, dense vegetation and traditional, antique Asian buildings with red roofs and wooden elements, perhaps from Thailand or Southeast Asia.  On the water, among the leaves, an old wooden boat is visible.  The lighting is soft, diffused, perhaps a cloudy day or early morning.",
    "gender": "female",
    "sort_order": 6
  },
  {
    "category_name": "Машина",
    "category_gender": "female",
    "name": "В машине",
    "prompt": "Use the attached face images as the exact identity reference and generate the same person. Do not change the shape, features and proportions of the face from the attached photographs. A cozy, intimate moment inside a modern, luxurious car with a panoramic glass roof, capturing a stunning snow-covered winter forest visible through the windows. The woman is seated in the passenger seat, turned slightly to the right, leaning back with her right elbow resting on the center console. Her right hand is now resting gently on her lap or casually by her side, not touching her head. Her expression is serene and pensive, looking out the window with softly parted lips. She is wearing an all-white, chunky cable-knit sweater and matching loose-fitting, high-waisted pants, creating a monochrome, soft texture contrast against the light grey leather car interior and a blanket covering the armrest. She is holding a light-colored, insulated travel mug in her left hand, bringing it up towards her chin. The natural light is soft, diffuse daylight, predominantly coming from the large side window and the sunroof, casting very gentle, high-key illumination with minimal shadows. The overall palette is a striking, clean monochrome of whites, light greys, and deep greens/blacks from the forest. The atmosphere is quiet, luxurious, and warmly contrasted with the cold winter outside.",
    "gender": "female",
    "sort_order": 1
  },
  {
    "category_name": "Машина",
    "category_gender": "female",
    "name": "Ночная поездка",
    "prompt": "Use the attached face images as the exact identity reference and generate the same person. Do not change the shape, features and proportions of the face from the attached photographs. Female model with confident party-girl expression. \nHead tilted slightly back and toward the camera. \nMouth open, tongue playfully sticking out, lips glossy. \nCheekbones highlighted, nose bridge slightly glowing, skin warm and luminous. \nExpression: bold, carefree, a bit provocative. \n \n[CAMERA / ANGLE] \nClose-up to medium shot from slightly below face level. \nCamera positioned outside the car, looking up toward the model leaning out of the window. \nMild wide-angle look (around 35mm) to emphasize perspective and nightlife energy. \nHorizontal framing. \n \n[POSE] \nModel sits inside the car, leaning out of the open window with upper body. \nLeft elbow resting on the car door edge, left shoulder closer to camera. \nLeft hand holding a martini glass toward the lens. \nRight shoulder slightly back, body twisted toward camera. \nNeck lengthened, hair falling over chest and coat. \n \n[OUTFIT] \nOversized faux-fur leopard coat: \n– rich white and beige tones \n– bold black spots \n– voluminous shoulders and sleeves \n– open collar showing a bit of darker inner clothing at neckline. \nInner clothing stays minimal and dark, so attention goes to coat. \n \n[HAIR / MAKEUP] \nHair: long, straight,  with slightly darker roots, length reaching below chest. \nStyling: loose, with a few front strands blown by wind, ends resting on coat. \nMakeup: \n– defined brows \n– smoky brown eye makeup \n– black eyeliner and mascara \n– warm bronzer and highlighter on cheekbones and nose \n– nude-pink glossy lips. \n \n[ACCESSORIES] \nBlack narrow rectangular sunglasses with dark opaque lenses. \nSeveral chunky silver rings on fingers holding the glass. \nThin silver bracelet on wrist. \nSmall silver hoop or stud earrings. \n \n[PROPS] \nClassic martini glass in hand: \n– transparent liquid with cold reflections \n– green olive (or two) on a cocktail pick. \nInterior of the car: glossy black door frame and window edge, slightly wet, reflecting city lights. \n \n[BACKGROUND] \nNight city Moscow",
    "gender": "female",
    "sort_order": 2
  },
  {
    "category_name": "Улица",
    "category_gender": "female",
    "name": "Девушка на балконе",
    "prompt": "Use the attached face images as the exact identity reference and generate the same person. Do not change the shape, features and proportions of the face from the attached photographs. The photo shows the woman from the uploaded photo, her face intact, with her hair tied up or combed back, sitting on the floor or low parapet.  She has an expressive, direct gaze.  The pose is sensual and thoughtful: she sits with her legs crossed at the knees and her arms clasped around them. \n Clothes and Style: \n The woman is dressed in translucent black tights or stockings, boots on a very high platform with a 90's style. On top of her is a loose dark leather jacket, which is casually thrown over her shoulders, partially exposing them.  The style is glamorous, fashionable, a little dark and dramatic. \n Setting and Background: \n • Setting: The scene takes place outdoors or on a balcony in the evening/night time. \n • Background: A dark metal railing or grate is visible behind, possibly a balcony or staircase.  Beyond the railing is a dark, blurry cityscape or background that adds depth. \n Light and Atmosphere: \n • Lighting: Backlighting and side lighting creates a strong dramatic mood.  The light source is probably located above or to the side, emphasizing the contours of the figure, the shine of the tights and the highlights on the leather jacket. \n • Color palette: Dark, monochrome or deep, muted shades of gray, black and dark brown dominate. \n • Atmosphere: Luxurious, mysterious, erotic, cinematic.",
    "gender": "female",
    "sort_order": 3
  },
  {
    "category_name": "Путешествия",
    "category_gender": "female",
    "name": "Нью-Йорк",
    "prompt": "[FACES — IDENTITY PRESERVATION]\nKeep the model’s face exactly as in the uploaded reference.\nNo changes to facial features, proportions, or natural expression.\n\n[CAMERA / ANGLE]\nFull-body street-style portrait.\nCamera at medium-low height for a slightly elongated fashion angle.\nFrontal-left perspective capturing both the model and the brownstone building behind her.\n\n[POSE]\nModel standing in the middle of a crosswalk.\nOne arm relaxed downward, the other holding a takeaway coffee cup near the chest.\nBody angled slightly to the side, head turned to the left with a soft, calm, observant expression.\n\n[OUTFIT]\nNeutral-toned casual chic:\n– long beige trench coat, open\n– white T-shirt tucked casually\n– loose straight blue jeans\n– white sneakers with soft gray details\n– dark baseball cap with minimalist logo\n– shoulder bag in textured brown leather\n\n[HAIR / MAKEUP]\nHair: natural smooth styling, straight with soft movement framing the face under the cap.\nMakeup:\n– natural matte coverage with soft highlight\n– subtle warm contour\n– neutral peach-nude lips\n– defined brows with natural fullness\n– light brown eyeshadow, minimal liner, soft lashes\nClean, everyday polished look.\n\n[BACKGROUND]\nClassic urban setting:\n– brownstone residential building facade\n– metal stair railings, dark doors\n– green plants and red flowers near the entrance\n– crosswalk markings on asphalt\n– warm afternoon light casting soft building shadows\n\n[LIGHTING]\nNatural outdoor daylight.\nSoft golden-hour illumination from the left side.\nGentle shadows across the building and pavement.\nClean, balanced exposure typical of lifestyle photography.\n\n[MOOD / AESTHETIC]\nCalm, minimal, effortlessly stylish New York street moment.\nCasual luxury with grounded realism.\n\n[OUTPUT]\nUltra-realistic full-body portrait.\nSharp detailing on fabrics (trench coat, denim texture, leather bag).\nClean skin texture, natural facial rendering, accurate proportions.\nEditorial street-style color grading with warm highlights and cool urban tones.",
    "gender": "female",
    "sort_order": 7
  },
  {
    "category_name": "Путешествия",
    "category_gender": "female",
    "name": "Европа",
    "prompt": "Use the attached face images as the exact identity reference and generate the same person. Do not change the shape, features and proportions of the face from the attached photographs. Hyper-realistic street-style portrait of the model sitting on a green metal bench along a European cobblestone street. She poses playfully, leaning her elbow on her knee while blowing a soft, flirty kiss toward the camera. Her hair is styled in a sleek high ponytail, showing off glowing bronzed skin, defined brows, soft brown/nude lipstick, and subtle glam makeup. She wears chunky gold hoop earrings and a minimal gold bracelet.\n\nHer outfit features an oversized vintage brown leather jacket layered over a white top, paired with relaxed light-wash denim jeans. She wears bold green-and-white Adidas Gazelle sneakers, adding a vivid pop of color. In one hand, she holds an iced coffee in a clear cup with a straw; beside her on the bench sits a small Louis Vuitton handbag with a structured shape and monogram pattern.\n\nThe background shows classic European architecture—colorful facades, ornate windows, small shops, and cobblestone sidewalks—along with cars, pedestrians, and soft overcast lighting that creates gentle shadows and natural tones. \n\nUltra-detailed textures: worn leather creases, denim grain, suede sneaker texture, glowing skin, iced coffee condensation, and architectural details in the background. Editorial lifestyle influencer energy with modern street aesthetic, Higgsfield hyper-realism",
    "gender": "female",
    "sort_order": 8
  },
  {
    "category_name": "Путешествия",
    "category_gender": "female",
    "name": "На подъемнике",
    "prompt": "Use the attached face images as the exact identity reference and generate the same person. Do not change the shape, features and proportions of the face from the attached photographs. A spontaneous medium shot captured from a slightly lower angle inside a ski lift gondola, featuring a woman sitting on a bright red bench. She wears a luxurious black faux-fur jacket with wide sleeves, sleek black fitted leggings, oversized black Moon Boot–style winter boots, black earmuffs, and glossy black rectangular sunglasses. One hand adjusts her earmuffs while the other casually holds a small black designer handbag near her knee. Her straight, smooth medium-length hair frames her face, which is calm, slightly pouty, and focused forward with detailed preservation of facial features. The panoramic windows behind reveal a snowy forest and mountain landscape softly illuminated by cold, natural daylight with natural reflections and diffused ambient cabin light. The sharp focus on her contrasts with the slightly blurred wintery background. The composition embodies authentic, casual iPhone photography spontaneity with crisp textures, reflective surfaces, and a relaxed yet confident posture, evoking a winter fashion editorial feel with clean, realistic visual tones.",
    "gender": "female",
    "sort_order": 9
  },
  {
    "category_name": "Путешествия",
    "category_gender": "female",
    "name": "Венеция",
    "prompt": "Use the attached face images as the exact identity reference and generate the same person. Do not change the shape, features and proportions of the face from the attached photos\nhyper realistic 8K.\n Scene: Woman sitting on a historic stone bridge in Venice, with canal in the background and gondolas passing by.  Athletic, elegant body, relaxed posture.\n Look: red bandana, round sunglasses, cream satin top with ruffles, long satin skirt.\n Wavy hair with light wind.\n\n Golden light from the sunset creating reflections in the water and a glow on the face.\n\n No text in the image.\n No visible tattoos.",
    "gender": "female",
    "sort_order": 10
  }
]$seed$::jsonb)
        AS x(category_name TEXT, category_gender TEXT, name TEXT, prompt TEXT, gender TEXT, sort_order INT)
),
resolved_categories AS (
    SELECT MIN(id) AS id, name, gender
    FROM categories
    GROUP BY name, gender
),
resolved_prompts AS (
    SELECT p.*, c.id AS category_id
    FROM prompt_seed AS p
    JOIN resolved_categories AS c
      ON c.name = p.category_name
     AND c.gender = p.category_gender
),
updated_prompts AS (
    UPDATE prompts AS p
    SET prompt = rp.prompt,
        sort_order = rp.sort_order,
        is_active = TRUE
    FROM resolved_prompts AS rp
    WHERE p.category_id = rp.category_id
      AND p.name = rp.name
      AND p.gender = rp.gender
    RETURNING p.id
)
INSERT INTO prompts (category_id, name, prompt, gender, sort_order, is_active)
SELECT rp.category_id, rp.name, rp.prompt, rp.gender, rp.sort_order, TRUE
FROM resolved_prompts AS rp
WHERE NOT EXISTS (
    SELECT 1
    FROM prompts AS p
    WHERE p.category_id = rp.category_id
      AND p.name = rp.name
      AND p.gender = rp.gender
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- No-op rollback: после импорта эти записи могут быть отредактированы через админку.
SELECT 1;
-- +goose StatementEnd
